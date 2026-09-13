package discount

import (
	"errors"
	"fmt"
)

// ErrInvalidRule is wrapped by every error returned from rule validation.
var ErrInvalidRule = errors.New("invalid discount rule")

// Validate reports whether the offer's terms are usable: it must have an ID
// and a name, a percentage in (0, 100], a non-negative cap and a validity
// window that ends after it starts.
func (o Offer) Validate() error {
	var errs []error
	if o.ID == "" {
		errs = append(errs, errors.New("id is required"))
	}
	if o.Name == "" {
		errs = append(errs, errors.New("name is required"))
	}
	if !o.Percentage.IsPositive() || o.Percentage.GreaterThan(hundred) {
		errs = append(errs, fmt.Errorf("percentage must be in (0, 100], got %s", o.Percentage))
	}
	if o.MaxAmount != nil && o.MaxAmount.IsNegative() {
		errs = append(errs, fmt.Errorf("max amount must not be negative, got %s", o.MaxAmount))
	}
	if !o.ValidFrom.IsZero() && !o.ValidUntil.IsZero() && o.ValidUntil.Before(o.ValidFrom) {
		errs = append(errs, errors.New("valid until is before valid from"))
	}
	return o.wrap(errors.Join(errs...))
}

// Validate implements Validator.
func (d BrandDiscount) Validate() error {
	return validateWith(d.Offer, d.Brand == "", "brand is required")
}

// Validate implements Validator.
func (d CategoryDiscount) Validate() error {
	return validateWith(d.Offer, d.Category == "", "category is required")
}

// Validate implements Validator.
func (v Voucher) Validate() error {
	return validateWith(v.Offer, NormalizeCode(v.Code) == "", "code is required")
}

// Validate implements Validator.
func (b BankOffer) Validate() error {
	return validateWith(b.Offer, b.BankName == "", "bank name is required")
}

// Validator is implemented by rules that can check their own configuration.
type Validator interface {
	Validate() error
}

func validateWith(o Offer, missing bool, msg string) error {
	err := o.Validate()
	if !missing {
		return err
	}
	return errors.Join(err, o.wrap(errors.New(msg)))
}

func (o Offer) wrap(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w %q: %w", ErrInvalidRule, o.ID, err)
}
