package service

import (
	"errors"
	"fmt"
)

// Sentinel errors returned (possibly wrapped) by the service. Use errors.Is to
// test for them.
var (
	ErrEmptyCart               = errors.New("cart is empty")
	ErrInvalidCartItem         = errors.New("invalid cart item")
	ErrEmptyDiscountCode       = errors.New("discount code is empty")
	ErrDiscountCodeNotFound    = errors.New("discount code not found")
	ErrDiscountNotYetActive    = errors.New("discount code is not yet active")
	ErrDiscountExpired         = errors.New("discount code has expired")
	ErrCustomerTierNotEligible = errors.New("customer tier not eligible")
	ErrNoEligibleItems         = errors.New("no eligible items in cart")
)

// ValidationError explains why a discount code cannot be applied. Reason is one
// of the sentinel errors above and is exposed through Unwrap.
type ValidationError struct {
	Code   string
	Reason error
	Detail string
}

func (e *ValidationError) Error() string {
	if e.Detail == "" {
		return fmt.Sprintf("discount code %q: %v", e.Code, e.Reason)
	}
	return fmt.Sprintf("discount code %q: %v: %s", e.Code, e.Reason, e.Detail)
}

// Unwrap returns the underlying sentinel reason.
func (e *ValidationError) Unwrap() error {
	return e.Reason
}
