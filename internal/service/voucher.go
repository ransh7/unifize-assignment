package service

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"strings"
	"time"

	"github.com/ransh7/unifize-assignment/internal/discount"
	"github.com/ransh7/unifize-assignment/internal/models"
	"github.com/ransh7/unifize-assignment/internal/repository"
)

// voucherChecker decides whether a voucher code can be used. It is shared by
// ValidateDiscountCode and the voucher pricing stage so both apply the same
// rules.
type voucherChecker struct {
	vouchers repository.VoucherSource
}

// Check looks up code and verifies, in order, that the voucher is active at
// now, that customer meets its tier requirement, and that at least one of
// products passes its brand exclusions and category restrictions. Business
// rule failures are reported as *ValidationError.
func (c voucherChecker) Check(ctx context.Context, code string, products iter.Seq[models.Product],
	customer models.CustomerProfile, now time.Time,
) (discount.Voucher, error) {
	code = discount.NormalizeCode(code)
	if code == "" {
		return discount.Voucher{}, &ValidationError{Code: code, Reason: ErrEmptyDiscountCode}
	}

	voucher, err := c.vouchers.VoucherByCode(ctx, code)
	if errors.Is(err, repository.ErrNotFound) {
		return discount.Voucher{}, &ValidationError{Code: code, Reason: ErrDiscountCodeNotFound}
	}
	if err != nil {
		return discount.Voucher{}, fmt.Errorf("looking up discount code %q: %w", code, err)
	}

	if err := checkVoucherRules(voucher, products, customer, now); err != nil {
		err.Code = code
		return discount.Voucher{}, err
	}
	return voucher, nil
}

func checkVoucherRules(v discount.Voucher, products iter.Seq[models.Product],
	customer models.CustomerProfile, now time.Time,
) *ValidationError {
	switch {
	case v.NotYetActive(now):
		return &ValidationError{
			Reason: ErrDiscountNotYetActive,
			Detail: "valid from " + v.ValidFrom.Format(time.RFC3339),
		}
	case v.Expired(now):
		return &ValidationError{
			Reason: ErrDiscountExpired,
			Detail: "expired at " + v.ValidUntil.Format(time.RFC3339),
		}
	case !customer.MeetsTier(v.MinCustomerTier):
		return &ValidationError{
			Reason: ErrCustomerTierNotEligible,
			Detail: fmt.Sprintf("requires %q tier or above, customer tier is %q", v.MinCustomerTier, customer.Tier),
		}
	}

	for p := range products {
		if v.AppliesToProduct(p) {
			return nil
		}
	}
	return &ValidationError{Reason: ErrNoEligibleItems, Detail: ineligibilityDetail(v)}
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

// productsOf yields the product of each cart item.
func productsOf(items []models.CartItem) iter.Seq[models.Product] {
	return func(yield func(models.Product) bool) {
		for i := range items {
			if !yield(items[i].Product) {
				return
			}
		}
	}
}
