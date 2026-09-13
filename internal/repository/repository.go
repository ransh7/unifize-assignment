// Package repository provides access to discount rule definitions.
package repository

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"strings"
	"time"

	"github.com/ransh7/unifize-assignment/internal/discount"
	"github.com/ransh7/unifize-assignment/internal/models"
)

// ErrNotFound is returned when a requested discount does not exist.
var ErrNotFound = errors.New("not found")

// ProductDiscountSource supplies product-level discounts.
//
// Lookups take the products being priced so an implementation only returns
// rules that can apply to them (e.g. WHERE brand IN (...)) instead of the
// whole catalogue of offers.
type ProductDiscountSource interface {
	// BrandDiscounts returns brand discounts active at t that target the
	// brand of at least one of products.
	BrandDiscounts(ctx context.Context, products iter.Seq[models.Product], t time.Time) ([]discount.BrandDiscount, error)
	// CategoryDiscounts returns category discounts active at t that target
	// the category of at least one of products.
	CategoryDiscounts(ctx context.Context, products iter.Seq[models.Product], t time.Time) ([]discount.CategoryDiscount, error)
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

// InMemoryRepository is a DiscountRepository that indexes rules in memory by
// the attribute they target. It is safe for concurrent use because it is
// never mutated after construction.
type InMemoryRepository struct {
	brands     map[string][]discount.BrandDiscount    // keyed by lower-case brand
	categories map[string][]discount.CategoryDiscount // keyed by lower-case category
	vouchers   map[string]discount.Voucher            // keyed by normalised code
	bankOffers []discount.BankOffer
}

var _ DiscountRepository = (*InMemoryRepository)(nil)

// NewInMemoryRepository returns a repository serving rules. It returns an
// error wrapping discount.ErrInvalidRule if any rule is misconfigured.
func NewInMemoryRepository(rules Rules) (*InMemoryRepository, error) {
	if err := rules.Validate(); err != nil {
		return nil, err
	}
	r := &InMemoryRepository{
		brands:     indexBy(rules.Brands, func(d discount.BrandDiscount) string { return d.Brand }),
		categories: indexBy(rules.Categories, func(d discount.CategoryDiscount) string { return d.Category }),
		vouchers:   make(map[string]discount.Voucher, len(rules.Vouchers)),
		bankOffers: rules.BankOffers,
	}
	for _, v := range rules.Vouchers {
		r.vouchers[discount.NormalizeCode(v.Code)] = v
	}
	return r, nil
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
func (r *InMemoryRepository) BrandDiscounts(ctx context.Context, products iter.Seq[models.Product], t time.Time) ([]discount.BrandDiscount, error) {
	return lookup(ctx, r.brands, products, func(p models.Product) string { return p.Brand }, t)
}

// CategoryDiscounts implements ProductDiscountSource.
func (r *InMemoryRepository) CategoryDiscounts(ctx context.Context, products iter.Seq[models.Product], t time.Time) ([]discount.CategoryDiscount, error) {
	return lookup(ctx, r.categories, products, func(p models.Product) string { return p.Category }, t)
}

// BankOffers implements BankOfferSource.
func (r *InMemoryRepository) BankOffers(ctx context.Context, t time.Time) ([]discount.BankOffer, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var out []discount.BankOffer
	for _, o := range r.bankOffers {
		if o.IsActiveAt(t) {
			out = append(out, o)
		}
	}
	return out, nil
}

// VoucherByCode implements VoucherSource.
func (r *InMemoryRepository) VoucherByCode(ctx context.Context, code string) (discount.Voucher, error) {
	if err := ctx.Err(); err != nil {
		return discount.Voucher{}, err
	}
	v, ok := r.vouchers[discount.NormalizeCode(code)]
	if !ok {
		return discount.Voucher{}, ErrNotFound
	}
	return v, nil
}

// indexBy groups rules by the lower-cased value of key.
func indexBy[R any](rules []R, key func(R) string) map[string][]R {
	index := make(map[string][]R)
	for _, rule := range rules {
		k := strings.ToLower(key(rule))
		index[k] = append(index[k], rule)
	}
	return index
}

// lookup returns the rules in index active at t whose key matches the key of
// at least one product. Each key is looked up once however many products
// share it.
func lookup[R discount.Rule](ctx context.Context, index map[string][]R, products iter.Seq[models.Product],
	key func(models.Product) string, t time.Time,
) ([]R, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var out []R
	seen := make(map[string]bool)
	for p := range products {
		k := strings.ToLower(key(p))
		if seen[k] {
			continue
		}
		seen[k] = true
		for _, rule := range index[k] {
			if rule.Terms().IsActiveAt(t) {
				out = append(out, rule)
			}
		}
	}
	return out, nil
}
