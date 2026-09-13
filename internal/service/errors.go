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

// ValidationError explains why a discount code cannot be applied.
type ValidationError struct {
	Code   string // normalised code the customer entered
	Reason error  // one of the Err* sentinels above, exposed through Unwrap
	Detail string // customer-facing specifics, e.g. the required tier
}

// Error formats the failure as `discount code "CODE": reason: detail`,
// omitting the parts that are empty.
func (e *ValidationError) Error() string {
	msg := e.Reason.Error()
	if e.Code != "" {
		msg = fmt.Sprintf("discount code %q: %s", e.Code, msg)
	}
	if e.Detail != "" {
		msg += ": " + e.Detail
	}
	return msg
}

// Unwrap returns the underlying sentinel reason.
func (e *ValidationError) Unwrap() error {
	return e.Reason
}
