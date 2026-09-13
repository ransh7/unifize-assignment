// Package repository provides access to discount rule definitions.
package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/ransh7/unifize-assignment/internal/models"
)

// ErrNotFound is returned when a requested discount does not exist.
var ErrNotFound = errors.New("not found")

// DiscountRepository is the source of discount rules used by the service.
type DiscountRepository interface {
	// ActiveDiscounts returns all discounts of the given type that are active at t.
	ActiveDiscounts(ctx context.Context, discountType models.DiscountType, t time.Time) ([]models.Discount, error)

	// VoucherByCode returns the voucher with the given code regardless of its
	// validity window, so callers can explain why a code cannot be used.
	// Lookup is case-insensitive. It returns ErrNotFound if no voucher matches.
	VoucherByCode(ctx context.Context, code string) (*models.Discount, error)
}

// InMemoryRepository is a DiscountRepository backed by a slice. It is safe for
// concurrent reads because it is never mutated after construction.
type InMemoryRepository struct {
	discounts []models.Discount
}

var _ DiscountRepository = (*InMemoryRepository)(nil)

// NewInMemoryRepository returns a repository serving the given discounts.
func NewInMemoryRepository(discounts []models.Discount) *InMemoryRepository {
	cp := make([]models.Discount, len(discounts))
	copy(cp, discounts)
	return &InMemoryRepository{discounts: cp}
}

// ActiveDiscounts implements DiscountRepository.
func (r *InMemoryRepository) ActiveDiscounts(ctx context.Context, discountType models.DiscountType, t time.Time) ([]models.Discount, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var out []models.Discount
	for _, d := range r.discounts {
		if d.Type == discountType && d.IsActiveAt(t) {
			out = append(out, d)
		}
	}
	return out, nil
}

// VoucherByCode implements DiscountRepository.
func (r *InMemoryRepository) VoucherByCode(ctx context.Context, code string) (*models.Discount, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for _, d := range r.discounts {
		if d.Type == models.DiscountTypeVoucher && strings.EqualFold(d.Code, code) {
			v := d
			return &v, nil
		}
	}
	return nil, ErrNotFound
}
