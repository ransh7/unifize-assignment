package service_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/ransh7/unifize-assignment/internal/discount"
	"github.com/ransh7/unifize-assignment/internal/models"
	"github.com/ransh7/unifize-assignment/internal/repository"
	"github.com/ransh7/unifize-assignment/internal/service"
	"github.com/ransh7/unifize-assignment/testdata"
)

// BenchmarkCalculateCartDiscounts prices a 20-line cart against a catalogue
// sized like a busy sale: hundreds of brand and voucher rules.
func BenchmarkCalculateCartDiscounts(b *testing.B) {
	const (
		brands     = 300
		categories = 50
		vouchers   = 500
		lines      = 20
	)

	rules := testdata.Rules()
	for i := range brands {
		rules.Brands = append(rules.Brands, discount.BrandDiscount{
			Offer: discount.Offer{ID: fmt.Sprintf("b%d", i), Name: fmt.Sprintf("Brand %d", i), Percentage: decimal.NewFromInt(20)},
			Brand: fmt.Sprintf("brand-%d", i),
		})
	}
	for i := range categories {
		rules.Categories = append(rules.Categories, discount.CategoryDiscount{
			Offer:    discount.Offer{ID: fmt.Sprintf("c%d", i), Name: fmt.Sprintf("Category %d", i), Percentage: decimal.NewFromInt(5)},
			Category: fmt.Sprintf("category-%d", i),
		})
	}
	for i := range vouchers {
		rules.Vouchers = append(rules.Vouchers, discount.Voucher{
			Offer: discount.Offer{ID: fmt.Sprintf("v%d", i), Name: fmt.Sprintf("Voucher %d", i), Percentage: decimal.NewFromInt(10)},
			Code:  fmt.Sprintf("CODE%d", i),
		})
	}
	repo, err := repository.NewInMemoryRepository(rules)
	if err != nil {
		b.Fatal(err)
	}
	svc := service.NewDiscountService(repo)

	cart := make([]models.CartItem, lines)
	for i := range cart {
		p := testdata.PumaTShirt
		p.Brand = fmt.Sprintf("brand-%d", i*7)
		p.Category = fmt.Sprintf("category-%d", i)
		cart[i] = models.CartItem{Product: p, Quantity: 1 + i%3}
	}
	ctx := service.WithVoucherCode(context.Background(), fmt.Sprintf("CODE%d", vouchers-1))

	b.ReportAllocs()
	for b.Loop() {
		if _, err := svc.CalculateCartDiscounts(ctx, cart, testdata.RegularCustomer, testdata.ICICICreditCard); err != nil {
			b.Fatal(err)
		}
	}
}
