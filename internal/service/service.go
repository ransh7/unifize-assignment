// Package service implements cart pricing and discount-code validation for the
// fashion e-commerce storefront.
package service

import (
	"context"
	"strings"

	"github.com/ransh7/unifize-assignment/internal/models"
)

// DiscountService prices carts and validates discount codes.
type DiscountService interface {
	// CalculateCartDiscounts calculates final price after applying discount logic:
	// - First apply brand/category discounts
	// - Then apply coupon codes
	// - Then apply bank offers
	//
	// A voucher code is supplied through the context with WithVoucherCode.
	CalculateCartDiscounts(ctx context.Context, cartItems []models.CartItem,
		customer models.CustomerProfile, paymentInfo *models.PaymentInfo) (*models.DiscountedPrice, error)

	// ValidateDiscountCode validates if a discount code can be applied.
	// Handle specific cases like:
	// - Brand exclusions
	// - Category restrictions
	// - Customer tier requirements
	//
	// When the code cannot be applied it returns false and a *ValidationError
	// describing why.
	ValidateDiscountCode(ctx context.Context, code string, cartItems []models.CartItem,
		customer models.CustomerProfile) (bool, error)
}

type voucherCodeKey struct{}

// WithVoucherCode returns a context carrying a voucher code to be applied by
// CalculateCartDiscounts.
func WithVoucherCode(ctx context.Context, code string) context.Context {
	return context.WithValue(ctx, voucherCodeKey{}, code)
}

// VoucherCodeFromContext returns the voucher code stored in ctx, if any.
func VoucherCodeFromContext(ctx context.Context) string {
	code, _ := ctx.Value(voucherCodeKey{}).(string)
	return strings.TrimSpace(code)
}
