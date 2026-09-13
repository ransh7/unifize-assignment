package repository_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/ransh7/unifize-assignment/internal/discount"
	"github.com/ransh7/unifize-assignment/internal/models"
	"github.com/ransh7/unifize-assignment/internal/repository"
	"github.com/ransh7/unifize-assignment/testdata"
)

func offer(id, name string, pct int64) discount.Offer {
	return discount.Offer{ID: id, Name: name, Percentage: decimal.NewFromInt(pct)}
}

func TestNewInMemoryRepositoryValidation(t *testing.T) {
	negativeCap := decimal.NewFromInt(-1)

	tests := []struct {
		name    string
		rules   repository.Rules
		wantMsg string // substring expected in the error; empty means valid
	}{
		{
			name:  "fake data is valid",
			rules: testdata.Rules(),
		},
		{
			name: "zero percentage",
			rules: repository.Rules{Brands: []discount.BrandDiscount{
				{Offer: offer("b1", "Brand", 0), Brand: "PUMA"},
			}},
			wantMsg: "percentage must be in (0, 100]",
		},
		{
			name: "percentage above 100",
			rules: repository.Rules{Categories: []discount.CategoryDiscount{
				{Offer: offer("c1", "Category", 101), Category: "T-shirts"},
			}},
			wantMsg: "percentage must be in (0, 100]",
		},
		{
			name: "negative cap",
			rules: repository.Rules{BankOffers: []discount.BankOffer{{
				Offer:    discount.Offer{ID: "k1", Name: "Bank", Percentage: decimal.NewFromInt(10), MaxAmount: &negativeCap},
				BankName: "ICICI",
			}}},
			wantMsg: "max amount must not be negative",
		},
		{
			name: "validity window ends before it starts",
			rules: repository.Rules{Brands: []discount.BrandDiscount{{
				Offer: discount.Offer{
					ID: "b1", Name: "Brand", Percentage: decimal.NewFromInt(10),
					ValidFrom:  time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
					ValidUntil: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				},
				Brand: "PUMA",
			}}},
			wantMsg: "valid until is before valid from",
		},
		{
			name: "missing targeting fields",
			rules: repository.Rules{
				Brands:     []discount.BrandDiscount{{Offer: offer("b1", "Brand", 10)}},
				Categories: []discount.CategoryDiscount{{Offer: offer("c1", "Category", 10)}},
				Vouchers:   []discount.Voucher{{Offer: offer("v1", "Voucher", 10), Code: "  "}},
				BankOffers: []discount.BankOffer{{Offer: offer("k1", "Bank", 10)}},
			},
			wantMsg: "brand is required",
		},
		{
			name: "duplicate names across rule types",
			rules: repository.Rules{
				Brands:     []discount.BrandDiscount{{Offer: offer("b1", "Sale", 40), Brand: "PUMA"}},
				Categories: []discount.CategoryDiscount{{Offer: offer("c1", "Sale", 10), Category: "T-shirts"}},
			},
			wantMsg: `duplicate name "Sale"`,
		},
		{
			name: "duplicate ids",
			rules: repository.Rules{
				Brands:     []discount.BrandDiscount{{Offer: offer("x", "Brand", 40), Brand: "PUMA"}},
				Categories: []discount.CategoryDiscount{{Offer: offer("x", "Category", 10), Category: "T-shirts"}},
			},
			wantMsg: `duplicate id "x"`,
		},
		{
			name: "duplicate voucher codes ignoring case",
			rules: repository.Rules{Vouchers: []discount.Voucher{
				{Offer: offer("v1", "One", 10), Code: "SAVE"},
				{Offer: offer("v2", "Two", 20), Code: " save "},
			}},
			wantMsg: `duplicate voucher code "SAVE"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, err := repository.NewInMemoryRepository(tt.rules)

			if tt.wantMsg == "" {
				if err != nil || repo == nil {
					t.Fatalf("NewInMemoryRepository = (%v, %v), want repository and nil error", repo, err)
				}
				return
			}

			if !errors.Is(err, discount.ErrInvalidRule) {
				t.Fatalf("error = %v, want wrapping discount.ErrInvalidRule", err)
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("error = %q, want it to contain %q", err, tt.wantMsg)
			}
			if repo != nil {
				t.Error("repository returned alongside error")
			}
		})
	}
}

func TestInMemoryRepositoryLookups(t *testing.T) {
	repo, err := repository.NewInMemoryRepository(testdata.Rules())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	t.Run("voucher lookup ignores case and spaces", func(t *testing.T) {
		v, err := repo.VoucherByCode(ctx, " super69 ")
		if err != nil || v.ID != testdata.Super69Voucher.ID {
			t.Fatalf("VoucherByCode = (%v, %v), want %s", v.ID, err, testdata.Super69Voucher.ID)
		}
	})

	t.Run("unknown voucher", func(t *testing.T) {
		if _, err := repo.VoucherByCode(ctx, "NOPE"); !errors.Is(err, repository.ErrNotFound) {
			t.Fatalf("error = %v, want ErrNotFound", err)
		}
	})

	t.Run("expired voucher is still returned so callers can explain", func(t *testing.T) {
		if _, err := repo.VoucherByCode(ctx, testdata.Expired10Voucher.Code); err != nil {
			t.Fatalf("error = %v, want nil", err)
		}
	})

	t.Run("product lookups only return rules targeting the products", func(t *testing.T) {
		products := slices.Values([]models.Product{testdata.PumaTShirt, testdata.PumaTShirt, testdata.NikeRunningShoes})

		brands, err := repo.BrandDiscounts(ctx, products, testdata.Now)
		if err != nil {
			t.Fatal(err)
		}
		if len(brands) != 1 || brands[0].ID != testdata.PumaBrandDiscount.ID {
			t.Errorf("BrandDiscounts = %v, want only %s once", brands, testdata.PumaBrandDiscount.ID)
		}

		none, err := repo.CategoryDiscounts(ctx, slices.Values([]models.Product{testdata.RoadsterJeans}), testdata.Now)
		if err != nil {
			t.Fatal(err)
		}
		if len(none) != 0 {
			t.Errorf("CategoryDiscounts for jeans = %v, want none", none)
		}
	})

	t.Run("active filters by validity window", func(t *testing.T) {
		limited := testdata.PumaBrandDiscount
		limited.ID, limited.Name = "limited", "Limited"
		limited.ValidUntil = testdata.Now.Add(-time.Hour)

		rules := testdata.Rules()
		rules.Brands = append(rules.Brands, limited)
		repo, err := repository.NewInMemoryRepository(rules)
		if err != nil {
			t.Fatal(err)
		}

		got, err := repo.BrandDiscounts(ctx, slices.Values([]models.Product{testdata.PumaTShirt}), testdata.Now)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].ID != testdata.PumaBrandDiscount.ID {
			t.Errorf("BrandDiscounts = %v, want only %s", got, testdata.PumaBrandDiscount.ID)
		}
	})
}
