package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/ransh7/unifize-assignment/internal/models"
	"github.com/ransh7/unifize-assignment/internal/repository"
	"github.com/ransh7/unifize-assignment/internal/service"
	"github.com/ransh7/unifize-assignment/testdata"
)

func newService() service.DiscountService {
	repo := repository.NewInMemoryRepository(testdata.AllDiscounts())
	return service.NewDiscountService(repo, service.WithClock(func() time.Time { return testdata.Now }))
}

func item(p models.Product, qty int) models.CartItem {
	return models.CartItem{Product: p, Quantity: qty, Size: "M"}
}

func dec(s string) decimal.Decimal {
	return decimal.RequireFromString(s)
}

func TestCalculateCartDiscounts(t *testing.T) {
	tests := []struct {
		name        string
		items       []models.CartItem
		customer    models.CustomerProfile
		payment     *models.PaymentInfo
		voucher     string
		wantOrig    string
		wantFinal   string
		wantApplied map[string]string
	}{
		{
			name:      "PUMA T-shirt with brand, category and ICICI bank offer",
			items:     []models.CartItem{item(testdata.PumaTShirt, 1)},
			customer:  testdata.RegularCustomer,
			payment:   testdata.ICICICreditCard,
			wantOrig:  "1000",
			wantFinal: "486",
			wantApplied: map[string]string{
				testdata.PumaBrandDiscount.Name:      "400",
				testdata.TShirtCategoryDiscount.Name: "60",
				testdata.ICICIBankOffer.Name:         "54",
			},
		},
		{
			name:      "quantity multiplies line price",
			items:     []models.CartItem{item(testdata.PumaTShirt, 2)},
			customer:  testdata.RegularCustomer,
			payment:   testdata.ICICICreditCard,
			wantOrig:  "2000",
			wantFinal: "972",
			wantApplied: map[string]string{
				testdata.PumaBrandDiscount.Name:      "800",
				testdata.TShirtCategoryDiscount.Name: "120",
				testdata.ICICIBankOffer.Name:         "108",
			},
		},
		{
			name:      "bank offer applies to whole cart total",
			items:     []models.CartItem{item(testdata.PumaTShirt, 1), item(testdata.NikeRunningShoes, 1)},
			customer:  testdata.RegularCustomer,
			payment:   testdata.ICICICreditCard,
			wantOrig:  "6000",
			wantFinal: "4986",
			wantApplied: map[string]string{
				testdata.PumaBrandDiscount.Name:      "400",
				testdata.TShirtCategoryDiscount.Name: "60",
				testdata.ICICIBankOffer.Name:         "554",
			},
		},
		{
			name:      "no bank offer for other bank",
			items:     []models.CartItem{item(testdata.PumaTShirt, 1)},
			customer:  testdata.RegularCustomer,
			payment:   testdata.HDFCCreditCard,
			wantOrig:  "1000",
			wantFinal: "540",
			wantApplied: map[string]string{
				testdata.PumaBrandDiscount.Name:      "400",
				testdata.TShirtCategoryDiscount.Name: "60",
			},
		},
		{
			name:      "no bank offer for ICICI UPI since offer is card only",
			items:     []models.CartItem{item(testdata.PumaTShirt, 1)},
			customer:  testdata.RegularCustomer,
			payment:   testdata.ICICIUPI,
			wantOrig:  "1000",
			wantFinal: "540",
			wantApplied: map[string]string{
				testdata.PumaBrandDiscount.Name:      "400",
				testdata.TShirtCategoryDiscount.Name: "60",
			},
		},
		{
			name:      "nil payment info skips bank offers",
			items:     []models.CartItem{item(testdata.PumaTShirt, 1)},
			customer:  testdata.RegularCustomer,
			wantOrig:  "1000",
			wantFinal: "540",
			wantApplied: map[string]string{
				testdata.PumaBrandDiscount.Name:      "400",
				testdata.TShirtCategoryDiscount.Name: "60",
			},
		},
		{
			name:      "voucher applied after brand/category and before bank offer",
			items:     []models.CartItem{item(testdata.PumaTShirt, 1)},
			customer:  testdata.RegularCustomer,
			payment:   testdata.ICICICreditCard,
			voucher:   "SUPER69",
			wantOrig:  "1000",
			wantFinal: "150.66",
			wantApplied: map[string]string{
				testdata.PumaBrandDiscount.Name:      "400",
				testdata.TShirtCategoryDiscount.Name: "60",
				testdata.Super69Voucher.Name:         "372.6",
				testdata.ICICIBankOffer.Name:         "16.74",
			},
		},
		{
			name:      "voucher skips excluded brand lines",
			items:     []models.CartItem{item(testdata.PumaTShirt, 1), item(testdata.NikeRunningShoes, 1)},
			customer:  testdata.RegularCustomer,
			voucher:   "SALE25",
			wantOrig:  "6000",
			wantFinal: "5405",
			wantApplied: map[string]string{
				testdata.PumaBrandDiscount.Name:      "400",
				testdata.TShirtCategoryDiscount.Name: "60",
				testdata.Sale25Voucher.Name:          "135",
			},
		},
		{
			name:        "product without any discounts",
			items:       []models.CartItem{item(testdata.RoadsterJeans, 1)},
			customer:    testdata.RegularCustomer,
			wantOrig:    "1500",
			wantFinal:   "1500",
			wantApplied: map[string]string{},
		},
	}

	svc := newService()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.voucher != "" {
				ctx = service.WithVoucherCode(ctx, tt.voucher)
			}

			got, err := svc.CalculateCartDiscounts(ctx, tt.items, tt.customer, tt.payment)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !got.OriginalPrice.Equal(dec(tt.wantOrig)) {
				t.Errorf("OriginalPrice = %s, want %s", got.OriginalPrice, tt.wantOrig)
			}
			if !got.FinalPrice.Equal(dec(tt.wantFinal)) {
				t.Errorf("FinalPrice = %s, want %s", got.FinalPrice, tt.wantFinal)
			}
			if len(got.AppliedDiscounts) != len(tt.wantApplied) {
				t.Errorf("AppliedDiscounts = %v, want %v", got.AppliedDiscounts, tt.wantApplied)
			}
			for name, want := range tt.wantApplied {
				if amount, ok := got.AppliedDiscounts[name]; !ok || !amount.Equal(dec(want)) {
					t.Errorf("AppliedDiscounts[%q] = %s (present=%v), want %s", name, amount, ok, want)
				}
			}
			if got.Message == "" {
				t.Error("Message is empty")
			}
		})
	}
}

func TestCalculateCartDiscountsErrors(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name    string
		ctx     context.Context
		items   []models.CartItem
		wantErr error
	}{
		{
			name:    "empty cart",
			ctx:     context.Background(),
			wantErr: service.ErrEmptyCart,
		},
		{
			name:    "zero quantity",
			ctx:     context.Background(),
			items:   []models.CartItem{item(testdata.PumaTShirt, 0)},
			wantErr: service.ErrInvalidCartItem,
		},
		{
			name:    "unknown voucher",
			ctx:     service.WithVoucherCode(context.Background(), "NOPE"),
			items:   []models.CartItem{item(testdata.PumaTShirt, 1)},
			wantErr: service.ErrDiscountCodeNotFound,
		},
		{
			name:    "canceled context",
			ctx:     canceled,
			items:   []models.CartItem{item(testdata.PumaTShirt, 1)},
			wantErr: context.Canceled,
		},
	}

	svc := newService()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := svc.CalculateCartDiscounts(tt.ctx, tt.items, testdata.RegularCustomer, nil)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if got != nil {
				t.Errorf("result = %+v, want nil", got)
			}
		})
	}
}

func TestValidateDiscountCode(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		items    []models.CartItem
		customer models.CustomerProfile
		wantErr  error
	}{
		{
			name:     "valid voucher on any product",
			code:     "SUPER69",
			items:    []models.CartItem{item(testdata.PumaTShirt, 1)},
			customer: testdata.RegularCustomer,
		},
		{
			name:     "code lookup is case-insensitive and trimmed",
			code:     "  super69 ",
			items:    []models.CartItem{item(testdata.PumaTShirt, 1)},
			customer: testdata.RegularCustomer,
		},
		{
			name:     "empty code",
			code:     "   ",
			items:    []models.CartItem{item(testdata.PumaTShirt, 1)},
			customer: testdata.RegularCustomer,
			wantErr:  service.ErrEmptyDiscountCode,
		},
		{
			name:     "unknown code",
			code:     "NOPE",
			items:    []models.CartItem{item(testdata.PumaTShirt, 1)},
			customer: testdata.RegularCustomer,
			wantErr:  service.ErrDiscountCodeNotFound,
		},
		{
			name:     "expired code",
			code:     "EXPIRED10",
			items:    []models.CartItem{item(testdata.PumaTShirt, 1)},
			customer: testdata.RegularCustomer,
			wantErr:  service.ErrDiscountExpired,
		},
		{
			name:     "customer tier too low",
			code:     "GOLD20",
			items:    []models.CartItem{item(testdata.PumaTShirt, 1)},
			customer: testdata.RegularCustomer,
			wantErr:  service.ErrCustomerTierNotEligible,
		},
		{
			name:     "customer tier satisfied",
			code:     "GOLD20",
			items:    []models.CartItem{item(testdata.PumaTShirt, 1)},
			customer: testdata.GoldCustomer,
		},
		{
			name:     "category restriction not met",
			code:     "TSHIRT15",
			items:    []models.CartItem{item(testdata.NikeRunningShoes, 1)},
			customer: testdata.RegularCustomer,
			wantErr:  service.ErrNoEligibleItems,
		},
		{
			name:     "category restriction met",
			code:     "TSHIRT15",
			items:    []models.CartItem{item(testdata.NikeRunningShoes, 1), item(testdata.PumaTShirt, 1)},
			customer: testdata.RegularCustomer,
		},
		{
			name:     "only excluded brands in cart",
			code:     "SALE25",
			items:    []models.CartItem{item(testdata.NikeRunningShoes, 1)},
			customer: testdata.RegularCustomer,
			wantErr:  service.ErrNoEligibleItems,
		},
		{
			name:     "excluded brand alongside eligible item",
			code:     "SALE25",
			items:    []models.CartItem{item(testdata.NikeRunningShoes, 1), item(testdata.RoadsterJeans, 1)},
			customer: testdata.RegularCustomer,
		},
		{
			name:     "empty cart",
			code:     "SUPER69",
			customer: testdata.RegularCustomer,
			wantErr:  service.ErrEmptyCart,
		},
	}

	svc := newService()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := svc.ValidateDiscountCode(context.Background(), tt.code, tt.items, tt.customer)

			if tt.wantErr == nil {
				if err != nil || !ok {
					t.Fatalf("ValidateDiscountCode = (%v, %v), want (true, nil)", ok, err)
				}
				return
			}

			if ok {
				t.Error("ok = true, want false")
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidationErrorDetail(t *testing.T) {
	svc := newService()
	_, err := svc.ValidateDiscountCode(context.Background(), "GOLD20",
		[]models.CartItem{item(testdata.PumaTShirt, 1)}, testdata.RegularCustomer)

	var vErr *service.ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("error = %v, want *service.ValidationError", err)
	}
	if vErr.Code != "GOLD20" || vErr.Detail == "" {
		t.Errorf("ValidationError = %+v, want code GOLD20 with detail", vErr)
	}
}
