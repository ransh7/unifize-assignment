package service

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"github.com/ransh7/unifize-assignment/internal/discount"
	"github.com/ransh7/unifize-assignment/internal/models"
	"github.com/ransh7/unifize-assignment/internal/pricing"
	"github.com/ransh7/unifize-assignment/internal/repository"
)

type discountService struct {
	vouchers voucherChecker
	pipeline pricing.Pipeline
	now      func() time.Time
}

var _ DiscountService = (*discountService)(nil)

// Option configures a DiscountService.
type Option func(*discountService)

// WithClock overrides the clock used to evaluate discount validity windows.
// A nil clock is ignored.
func WithClock(now func() time.Time) Option {
	return func(s *discountService) {
		if now != nil {
			s.now = now
		}
	}
}

// NewDiscountService returns a DiscountService that reads rules from repo.
//
// Discounts are applied in this order, each on the price left by the previous
// stage:
//  1. brand discounts (per line)
//  2. category discounts (per line)
//  3. voucher (on the eligible lines)
//  4. bank offer (on the cart total)
func NewDiscountService(repo repository.DiscountRepository, opts ...Option) DiscountService {
	vouchers := voucherChecker{vouchers: repo}
	s := &discountService{
		vouchers: vouchers,
		pipeline: pricing.NewPipeline(
			productDiscountStage[discount.BrandDiscount]{kind: "brand", load: repo.BrandDiscounts},
			productDiscountStage[discount.CategoryDiscount]{kind: "category", load: repo.CategoryDiscounts},
			voucherStage{vouchers: vouchers},
			bankOfferStage{offers: repo},
		),
		now: time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// CalculateCartDiscounts implements DiscountService.
func (s *discountService) CalculateCartDiscounts(ctx context.Context, cartItems []models.CartItem,
	customer models.CustomerProfile, paymentInfo *models.PaymentInfo,
) (*models.DiscountedPrice, error) {
	if err := validateCart(cartItems); err != nil {
		return nil, err
	}

	cart := pricing.NewCart(cartItems, customer, paymentInfo, s.now())
	cart.VoucherCode = VoucherCodeFromContext(ctx)

	if err := s.pipeline.Run(ctx, cart); err != nil {
		return nil, err
	}

	applied := cart.Applied()
	return &models.DiscountedPrice{
		OriginalPrice:    cart.Original(),
		FinalPrice:       cart.Total(),
		AppliedDiscounts: applied,
		Message:          summaryMessage(cart.Original(), cart.Total(), len(applied)),
	}, nil
}

// ValidateDiscountCode implements DiscountService.
func (s *discountService) ValidateDiscountCode(ctx context.Context, code string, cartItems []models.CartItem,
	customer models.CustomerProfile,
) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if err := validateCart(cartItems); err != nil {
		return false, err
	}
	if _, err := s.vouchers.Check(ctx, code, productsOf(cartItems), customer, s.now()); err != nil {
		return false, err
	}
	return true, nil
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
