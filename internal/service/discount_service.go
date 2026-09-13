package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/ransh7/unifize-assignment/internal/models"
	"github.com/ransh7/unifize-assignment/internal/repository"
)

var hundred = decimal.NewFromInt(100)

// moneyPlaces is the number of decimal places monetary amounts are rounded to.
const moneyPlaces = 2

type discountService struct {
	repo repository.DiscountRepository
	now  func() time.Time
}

var _ DiscountService = (*discountService)(nil)

// Option configures a DiscountService.
type Option func(*discountService)

// WithClock overrides the clock used to evaluate discount validity windows.
func WithClock(now func() time.Time) Option {
	return func(s *discountService) {
		s.now = now
	}
}

// NewDiscountService returns a DiscountService that reads rules from repo.
func NewDiscountService(repo repository.DiscountRepository, opts ...Option) DiscountService {
	s := &discountService{repo: repo, now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// cartLine tracks the running price of a single cart item across stages.
type cartLine struct {
	item  models.CartItem
	price decimal.Decimal
}

// CalculateCartDiscounts implements DiscountService.
func (s *discountService) CalculateCartDiscounts(ctx context.Context, cartItems []models.CartItem,
	customer models.CustomerProfile, paymentInfo *models.PaymentInfo,
) (*models.DiscountedPrice, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateCart(cartItems); err != nil {
		return nil, err
	}

	now := s.now()
	applied := make(map[string]decimal.Decimal)

	lines := make([]cartLine, len(cartItems))
	original := decimal.Zero
	for i, item := range cartItems {
		gross := item.Product.BasePrice.Mul(decimal.NewFromInt(int64(item.Quantity)))
		lines[i] = cartLine{item: item, price: gross}
		original = original.Add(gross)
	}

	// Stage 1: brand and category discounts, applied per line.
	brandDiscounts, err := s.repo.ActiveDiscounts(ctx, models.DiscountTypeBrand, now)
	if err != nil {
		return nil, fmt.Errorf("loading brand discounts: %w", err)
	}
	categoryDiscounts, err := s.repo.ActiveDiscounts(ctx, models.DiscountTypeCategory, now)
	if err != nil {
		return nil, fmt.Errorf("loading category discounts: %w", err)
	}
	for i := range lines {
		product := lines[i].item.Product

		if d, amount, ok := bestDiscount(brandDiscounts, lines[i].price, func(d models.Discount) bool {
			return strings.EqualFold(d.Brand, product.Brand)
		}); ok {
			lines[i].price = lines[i].price.Sub(amount)
			addApplied(applied, d.Name, amount)
		}

		if d, amount, ok := bestDiscount(categoryDiscounts, lines[i].price, func(d models.Discount) bool {
			return strings.EqualFold(d.Category, product.Category)
		}); ok {
			lines[i].price = lines[i].price.Sub(amount)
			addApplied(applied, d.Name, amount)
		}
	}

	total := decimal.Zero
	for _, l := range lines {
		total = total.Add(l.price)
	}

	// Stage 2: voucher, applied to the discounted price of eligible lines.
	if code := VoucherCodeFromContext(ctx); code != "" {
		voucher, err := s.checkVoucher(ctx, code, cartItems, customer, now)
		if err != nil {
			return nil, err
		}
		eligible := decimal.Zero
		for _, l := range lines {
			if voucherAppliesTo(voucher, l.item.Product) {
				eligible = eligible.Add(l.price)
			}
		}
		amount := discountAmount(*voucher, eligible)
		if amount.IsPositive() {
			total = total.Sub(amount)
			addApplied(applied, voucher.Name, amount)
		}
	}

	// Stage 3: bank offer, applied to the cart total.
	if paymentInfo != nil {
		offers, err := s.repo.ActiveDiscounts(ctx, models.DiscountTypeBankOffer, now)
		if err != nil {
			return nil, fmt.Errorf("loading bank offers: %w", err)
		}
		if d, amount, ok := bestDiscount(offers, total, func(d models.Discount) bool {
			return bankOfferMatches(d, paymentInfo)
		}); ok {
			total = total.Sub(amount)
			addApplied(applied, d.Name, amount)
		}
	}

	return &models.DiscountedPrice{
		OriginalPrice:    original,
		FinalPrice:       total,
		AppliedDiscounts: applied,
		Message:          summaryMessage(original, total, len(applied)),
	}, nil
}

// ValidateDiscountCode implements DiscountService.
func (s *discountService) ValidateDiscountCode(ctx context.Context, code string, cartItems []models.CartItem,
	customer models.CustomerProfile,
) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if _, err := s.checkVoucher(ctx, code, cartItems, customer, s.now()); err != nil {
		return false, err
	}
	return true, nil
}

// checkVoucher looks up a voucher and verifies it can be applied to the cart
// for the customer at time now.
func (s *discountService) checkVoucher(ctx context.Context, code string, cartItems []models.CartItem,
	customer models.CustomerProfile, now time.Time,
) (*models.Discount, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return nil, &ValidationError{Code: code, Reason: ErrEmptyDiscountCode}
	}
	if err := validateCart(cartItems); err != nil {
		return nil, err
	}

	voucher, err := s.repo.VoucherByCode(ctx, code)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, &ValidationError{Code: code, Reason: ErrDiscountCodeNotFound}
	}
	if err != nil {
		return nil, fmt.Errorf("looking up discount code %q: %w", code, err)
	}

	if !voucher.ValidFrom.IsZero() && now.Before(voucher.ValidFrom) {
		return nil, &ValidationError{
			Code:   code,
			Reason: ErrDiscountNotYetActive,
			Detail: "valid from " + voucher.ValidFrom.Format(time.RFC3339),
		}
	}
	if !voucher.ValidUntil.IsZero() && now.After(voucher.ValidUntil) {
		return nil, &ValidationError{
			Code:   code,
			Reason: ErrDiscountExpired,
			Detail: "expired at " + voucher.ValidUntil.Format(time.RFC3339),
		}
	}

	if !customer.MeetsTier(voucher.MinCustomerTier) {
		return nil, &ValidationError{
			Code:   code,
			Reason: ErrCustomerTierNotEligible,
			Detail: fmt.Sprintf("requires %q tier or above, customer tier is %q", voucher.MinCustomerTier, customer.Tier),
		}
	}

	for _, item := range cartItems {
		if voucherAppliesTo(voucher, item.Product) {
			return voucher, nil
		}
	}
	return nil, &ValidationError{Code: code, Reason: ErrNoEligibleItems, Detail: ineligibilityDetail(voucher)}
}

// voucherAppliesTo reports whether the voucher's brand exclusions and category
// restrictions allow it to discount product.
func voucherAppliesTo(v *models.Discount, p models.Product) bool {
	if slices.ContainsFunc(v.ExcludedBrands, func(b string) bool { return strings.EqualFold(b, p.Brand) }) {
		return false
	}
	if len(v.AllowedCategories) == 0 {
		return true
	}
	return slices.ContainsFunc(v.AllowedCategories, func(c string) bool { return strings.EqualFold(c, p.Category) })
}

func ineligibilityDetail(v *models.Discount) string {
	var parts []string
	if len(v.AllowedCategories) > 0 {
		parts = append(parts, "only valid on categories: "+strings.Join(v.AllowedCategories, ", "))
	}
	if len(v.ExcludedBrands) > 0 {
		parts = append(parts, "not valid on brands: "+strings.Join(v.ExcludedBrands, ", "))
	}
	return strings.Join(parts, "; ")
}

// bankOfferMatches reports whether a bank offer applies to the payment.
func bankOfferMatches(d models.Discount, p *models.PaymentInfo) bool {
	if d.PaymentMethod != "" && !strings.EqualFold(d.PaymentMethod, p.Method) {
		return false
	}
	if d.BankName != "" && (p.BankName == nil || !strings.EqualFold(d.BankName, *p.BankName)) {
		return false
	}
	if d.CardType != "" && (p.CardType == nil || !strings.EqualFold(d.CardType, *p.CardType)) {
		return false
	}
	return true
}

// bestDiscount returns the matching discount yielding the largest amount off
// price, along with that amount.
func bestDiscount(discounts []models.Discount, price decimal.Decimal, matches func(models.Discount) bool) (models.Discount, decimal.Decimal, bool) {
	var (
		best       models.Discount
		bestAmount decimal.Decimal
		found      bool
	)
	for _, d := range discounts {
		if !matches(d) {
			continue
		}
		amount := discountAmount(d, price)
		if !found || amount.GreaterThan(bestAmount) {
			best, bestAmount, found = d, amount, true
		}
	}
	return best, bestAmount, found
}

// discountAmount computes the rounded, capped discount d gives on price. The
// result never exceeds price.
func discountAmount(d models.Discount, price decimal.Decimal) decimal.Decimal {
	amount := price.Mul(d.Percentage).Div(hundred).Round(moneyPlaces)
	if d.MaxDiscountAmount != nil && amount.GreaterThan(*d.MaxDiscountAmount) {
		amount = *d.MaxDiscountAmount
	}
	if amount.GreaterThan(price) {
		amount = price
	}
	return amount
}

func addApplied(applied map[string]decimal.Decimal, name string, amount decimal.Decimal) {
	applied[name] = applied[name].Add(amount)
}

func summaryMessage(original, final decimal.Decimal, count int) string {
	if count == 0 {
		return "No discounts applied"
	}
	saved := original.Sub(final)
	pct := decimal.Zero
	if original.IsPositive() {
		pct = saved.Div(original).Mul(hundred).Round(moneyPlaces)
	}
	return fmt.Sprintf("Applied %d discount(s): you save %s (%s%%)", count, saved.StringFixed(moneyPlaces), pct.StringFixed(moneyPlaces))
}

func validateCart(cartItems []models.CartItem) error {
	if len(cartItems) == 0 {
		return ErrEmptyCart
	}
	for i, item := range cartItems {
		if item.Quantity <= 0 {
			return fmt.Errorf("%w: item %d (product %q) has non-positive quantity %d",
				ErrInvalidCartItem, i, item.Product.ID, item.Quantity)
		}
		if item.Product.BasePrice.IsNegative() {
			return fmt.Errorf("%w: item %d (product %q) has negative base price %s",
				ErrInvalidCartItem, i, item.Product.ID, item.Product.BasePrice)
		}
	}
	return nil
}
