package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/ransh7/unifize-assignment/internal/discount"
	"github.com/ransh7/unifize-assignment/internal/models"
	"github.com/ransh7/unifize-assignment/internal/repository"
)

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
	brandDiscounts, err := s.repo.BrandDiscounts(ctx, now)
	if err != nil {
		return nil, fmt.Errorf("loading brand discounts: %w", err)
	}
	categoryDiscounts, err := s.repo.CategoryDiscounts(ctx, now)
	if err != nil {
		return nil, fmt.Errorf("loading category discounts: %w", err)
	}
	for i := range lines {
		applyBestProductRule(&lines[i], brandDiscounts, applied)
		applyBestProductRule(&lines[i], categoryDiscounts, applied)
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
			if voucher.AppliesToProduct(l.item.Product) {
				eligible = eligible.Add(l.price)
			}
		}
		amount := voucher.AmountOff(eligible)
		if amount.IsPositive() {
			total = total.Sub(amount)
			addApplied(applied, voucher.Name, amount)
		}
	}

	// Stage 3: bank offer, applied to the cart total.
	if paymentInfo != nil {
		offers, err := s.repo.BankOffers(ctx, now)
		if err != nil {
			return nil, fmt.Errorf("loading bank offers: %w", err)
		}
		if offer, amount, ok := discount.Best(offers, total, func(o discount.BankOffer) bool {
			return o.AppliesToPayment(paymentInfo)
		}); ok {
			total = total.Sub(amount)
			addApplied(applied, offer.Name, amount)
		}
	}

	return &models.DiscountedPrice{
		OriginalPrice:    original,
		FinalPrice:       total,
		AppliedDiscounts: applied,
		Message:          summaryMessage(original, total, len(applied)),
	}, nil
}

// applyBestProductRule applies the most valuable rule that targets the line's
// product, if any.
func applyBestProductRule[R discount.ProductRule](line *cartLine, rules []R, applied map[string]decimal.Decimal) {
	rule, amount, ok := discount.Best(rules, line.price, func(r R) bool {
		return r.AppliesToProduct(line.item.Product)
	})
	if !ok {
		return
	}
	line.price = line.price.Sub(amount)
	addApplied(applied, rule.Terms().Name, amount)
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
) (discount.Voucher, error) {
	code = discount.NormalizeCode(code)
	if code == "" {
		return discount.Voucher{}, &ValidationError{Code: code, Reason: ErrEmptyDiscountCode}
	}
	if err := validateCart(cartItems); err != nil {
		return discount.Voucher{}, err
	}

	voucher, err := s.repo.VoucherByCode(ctx, code)
	if errors.Is(err, repository.ErrNotFound) {
		return discount.Voucher{}, &ValidationError{Code: code, Reason: ErrDiscountCodeNotFound}
	}
	if err != nil {
		return discount.Voucher{}, fmt.Errorf("looking up discount code %q: %w", code, err)
	}

	switch {
	case voucher.NotYetActive(now):
		return discount.Voucher{}, &ValidationError{
			Code:   code,
			Reason: ErrDiscountNotYetActive,
			Detail: "valid from " + voucher.ValidFrom.Format(time.RFC3339),
		}
	case voucher.Expired(now):
		return discount.Voucher{}, &ValidationError{
			Code:   code,
			Reason: ErrDiscountExpired,
			Detail: "expired at " + voucher.ValidUntil.Format(time.RFC3339),
		}
	case !customer.MeetsTier(voucher.MinCustomerTier):
		return discount.Voucher{}, &ValidationError{
			Code:   code,
			Reason: ErrCustomerTierNotEligible,
			Detail: fmt.Sprintf("requires %q tier or above, customer tier is %q", voucher.MinCustomerTier, customer.Tier),
		}
	}

	for _, item := range cartItems {
		if voucher.AppliesToProduct(item.Product) {
			return voucher, nil
		}
	}
	return discount.Voucher{}, &ValidationError{Code: code, Reason: ErrNoEligibleItems, Detail: ineligibilityDetail(voucher)}
}

func ineligibilityDetail(v discount.Voucher) string {
	var parts []string
	if len(v.AllowedCategories) > 0 {
		parts = append(parts, "only valid on categories: "+strings.Join(v.AllowedCategories, ", "))
	}
	if len(v.ExcludedBrands) > 0 {
		parts = append(parts, "not valid on brands: "+strings.Join(v.ExcludedBrands, ", "))
	}
	return strings.Join(parts, "; ")
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
		pct = saved.Div(original).Mul(decimal.NewFromInt(100)).Round(discount.MoneyPlaces)
	}
	return fmt.Sprintf("Applied %d discount(s): you save %s (%s%%)", count,
		saved.StringFixed(discount.MoneyPlaces), pct.StringFixed(discount.MoneyPlaces))
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
