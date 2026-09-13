// Package repository provides access to discount rule definitions.
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ransh7/unifize-assignment/internal/discount"
)

// ErrNotFound is returned when a requested discount does not exist.
var ErrNotFound = errors.New("not found")

// ProductDiscountSource supplies product-level discounts.
type ProductDiscountSource interface {
	// BrandDiscounts returns brand discounts active at t.
	BrandDiscounts(ctx context.Context, t time.Time) ([]discount.BrandDiscount, error)
	// CategoryDiscounts returns category discounts active at t.
	CategoryDiscounts(ctx context.Context, t time.Time) ([]discount.CategoryDiscount, error)
}

// VoucherSource looks up vouchers by code.
type VoucherSource interface {
	// VoucherByCode returns the voucher with the given code regardless of its
	// validity window, so callers can explain why a code cannot be used.
	// Lookup is case-insensitive. It returns ErrNotFound if no voucher matches.
	VoucherByCode(ctx context.Context, code string) (discount.Voucher, error)
}

// BankOfferSource supplies bank offers.
type BankOfferSource interface {
	// BankOffers returns bank offers active at t.
	BankOffers(ctx context.Context, t time.Time) ([]discount.BankOffer, error)
}

// DiscountRepository is the complete source of discount rules.
type DiscountRepository interface {
	ProductDiscountSource
	VoucherSource
	BankOfferSource
}

// Rules is the full set of discount rules served by an InMemoryRepository.
type Rules struct {
	Brands     []discount.BrandDiscount
	Categories []discount.CategoryDiscount
	Vouchers   []discount.Voucher
	BankOffers []discount.BankOffer
}

// InMemoryRepository is a DiscountRepository backed by slices. It is safe for
// concurrent use because it is never mutated after construction.
type InMemoryRepository struct {
	rules Rules
}

var _ DiscountRepository = (*InMemoryRepository)(nil)

// NewInMemoryRepository returns a repository serving rules. It returns an
// error wrapping discount.ErrInvalidRule if any rule is misconfigured.
func NewInMemoryRepository(rules Rules) (*InMemoryRepository, error) {
	if err := rules.Validate(); err != nil {
		return nil, err
	}
	return &InMemoryRepository{rules: rules}, nil
}

// Validate checks every rule and reports all problems at once. Besides each
// rule's own checks it requires:
//   - IDs to be unique across all rules
//   - names to be unique, since DiscountedPrice.AppliedDiscounts is keyed by
//     name and two rules sharing one would be silently merged
//   - voucher codes to be unique, ignoring case and surrounding spaces
func (r Rules) Validate() error {
	v := rulesValidator{ids: map[string]bool{}, names: map[string]bool{}, codes: map[string]bool{}}
	for _, d := range r.Brands {
		v.check(d)
	}
	for _, d := range r.Categories {
		v.check(d)
	}
	for _, d := range r.BankOffers {
		v.check(d)
	}
	for _, d := range r.Vouchers {
		v.check(d)
		v.checkCode(d.Code)
	}
	return errors.Join(v.errs...)
}

type validatableRule interface {
	discount.Rule
	discount.Validator
}

// rulesValidator accumulates validation errors across a set of rules.
type rulesValidator struct {
	ids, names, codes map[string]bool
	errs              []error
}

func (v *rulesValidator) check(rule validatableRule) {
	if err := rule.Validate(); err != nil {
		v.errs = append(v.errs, err)
	}
	terms := rule.Terms()
	v.unique(v.ids, "id", terms.ID)
	v.unique(v.names, "name", terms.Name)
}

func (v *rulesValidator) checkCode(code string) {
	v.unique(v.codes, "voucher code", discount.NormalizeCode(code))
}

func (v *rulesValidator) unique(seen map[string]bool, field, value string) {
	if value == "" {
		return // missing values are reported by the rule's own Validate
	}
	if seen[value] {
		v.errs = append(v.errs, fmt.Errorf("%w: duplicate %s %q", discount.ErrInvalidRule, field, value))
	}
	seen[value] = true
}

// BrandDiscounts implements ProductDiscountSource.
func (r *InMemoryRepository) BrandDiscounts(ctx context.Context, t time.Time) ([]discount.BrandDiscount, error) {
	return activeAt(ctx, r.rules.Brands, t)
}

// CategoryDiscounts implements ProductDiscountSource.
func (r *InMemoryRepository) CategoryDiscounts(ctx context.Context, t time.Time) ([]discount.CategoryDiscount, error) {
	return activeAt(ctx, r.rules.Categories, t)
}

// BankOffers implements BankOfferSource.
func (r *InMemoryRepository) BankOffers(ctx context.Context, t time.Time) ([]discount.BankOffer, error) {
	return activeAt(ctx, r.rules.BankOffers, t)
}

// VoucherByCode implements VoucherSource.
func (r *InMemoryRepository) VoucherByCode(ctx context.Context, code string) (discount.Voucher, error) {
	if err := ctx.Err(); err != nil {
		return discount.Voucher{}, err
	}
	code = discount.NormalizeCode(code)
	for _, v := range r.rules.Vouchers {
		if discount.NormalizeCode(v.Code) == code {
			return v, nil
		}
	}
	return discount.Voucher{}, ErrNotFound
}

func activeAt[R discount.Rule](ctx context.Context, rules []R, t time.Time) ([]R, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var out []R
	for _, r := range rules {
		if r.Terms().IsActiveAt(t) {
			out = append(out, r)
		}
	}
	return out, nil
}
