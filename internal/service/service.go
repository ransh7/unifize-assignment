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
	// CalculateCartDiscounts calculates the final price after applying
	// discounts in this order, each to the price left by the previous step:
	//  1. brand and category discounts, per line
	//  2. the voucher code, if one was attached to ctx with WithVoucherCode
	//  3. the best matching bank offer, if paymentInfo is non-nil
	//
	// An invalid voucher code fails the whole calculation with a
	// *ValidationError, so the customer is told why rather than silently
	// charged the undiscounted price.
	CalculateCartDiscounts(ctx context.Context, cartItems []models.CartItem,
		customer models.CustomerProfile, paymentInfo *models.PaymentInfo) (*models.DiscountedPrice, error)

	// ValidateDiscountCode reports whether a voucher code can be applied to
	// the cart by the customer. It checks that the code exists and is active,
	// that the customer meets its tier requirement, and that at least one item
	// passes its brand exclusions and category restrictions.
	//
	// When the code cannot be applied it returns false and a *ValidationError
	// whose Reason is one of the Err* sentinels. Other errors (a cancelled
	// context, an invalid cart, a repository failure) are returned as-is.
	ValidateDiscountCode(ctx context.Context, code string, cartItems []models.CartItem,
		customer models.CustomerProfile) (bool, error)
}

type voucherCodeKey struct{}

// WithVoucherCode returns a context carrying a voucher code to be applied by
// CalculateCartDiscounts.
//
// The prescribed CalculateCartDiscounts signature has no parameter for the
// code, so it travels as request-scoped data instead of changing the
// interface.
func WithVoucherCode(ctx context.Context, code string) context.Context {
	return context.WithValue(ctx, voucherCodeKey{}, code)
}

// voucherCodeFrom returns the voucher code stored in ctx, if any.
func voucherCodeFrom(ctx context.Context) string {
	code, _ := ctx.Value(voucherCodeKey{}).(string)
	return strings.TrimSpace(code)
}
