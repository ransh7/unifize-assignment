package discount_test

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/ransh7/unifize-assignment/internal/discount"
	"github.com/ransh7/unifize-assignment/internal/models"
	"github.com/ransh7/unifize-assignment/testdata"
)

func dec(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func ptr[T any](v T) *T { return &v }

func TestOfferAmountOff(t *testing.T) {
	tests := []struct {
		name  string
		offer discount.Offer
		price string
		want  string
	}{
		{name: "simple percentage", offer: discount.Offer{Percentage: dec("10")}, price: "540", want: "54"},
		{name: "rounds half away from zero to 2 places", offer: discount.Offer{Percentage: dec("15")}, price: "0.1", want: "0.02"},
		{name: "fractional percentage", offer: discount.Offer{Percentage: dec("12.5")}, price: "999.99", want: "125"},
		{name: "cap limits amount", offer: discount.Offer{Percentage: dec("10"), MaxAmount: ptr(dec("50"))}, price: "1000", want: "50"},
		{name: "cap above amount has no effect", offer: discount.Offer{Percentage: dec("10"), MaxAmount: ptr(dec("500"))}, price: "1000", want: "100"},
		{name: "never exceeds price", offer: discount.Offer{Percentage: dec("100")}, price: "12.34", want: "12.34"},
		{name: "zero price", offer: discount.Offer{Percentage: dec("50")}, price: "0", want: "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.offer.AmountOff(dec(tt.price)); !got.Equal(dec(tt.want)) {
				t.Errorf("AmountOff(%s) = %s, want %s", tt.price, got, tt.want)
			}
		})
	}
}

func TestOfferValidityWindow(t *testing.T) {
	now := testdata.Now
	tests := []struct {
		name                string
		offer               discount.Offer
		wantNotYet, wantExp bool
	}{
		{name: "open window", offer: discount.Offer{}},
		{name: "starts later", offer: discount.Offer{ValidFrom: now.Add(time.Hour)}, wantNotYet: true},
		{name: "ended", offer: discount.Offer{ValidUntil: now.Add(-time.Hour)}, wantExp: true},
		{name: "inclusive bounds", offer: discount.Offer{ValidFrom: now, ValidUntil: now}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.offer.NotYetActive(now); got != tt.wantNotYet {
				t.Errorf("NotYetActive = %v, want %v", got, tt.wantNotYet)
			}
			if got := tt.offer.Expired(now); got != tt.wantExp {
				t.Errorf("Expired = %v, want %v", got, tt.wantExp)
			}
			if got, want := tt.offer.IsActiveAt(now), !tt.wantNotYet && !tt.wantExp; got != want {
				t.Errorf("IsActiveAt = %v, want %v", got, want)
			}
		})
	}
}

func TestProductRulesAppliesToProduct(t *testing.T) {
	tests := []struct {
		name    string
		rule    discount.ProductRule
		product models.Product
		want    bool
	}{
		{name: "brand matches ignoring case", rule: discount.BrandDiscount{Brand: "puma"}, product: testdata.PumaTShirt, want: true},
		{name: "brand differs", rule: discount.BrandDiscount{Brand: "PUMA"}, product: testdata.NikeRunningShoes},
		{name: "category matches", rule: discount.CategoryDiscount{Category: "t-shirts"}, product: testdata.PumaTShirt, want: true},
		{name: "category differs", rule: discount.CategoryDiscount{Category: "T-shirts"}, product: testdata.RoadsterJeans},
		{name: "unrestricted voucher", rule: testdata.Super69Voucher, product: testdata.NikeRunningShoes, want: true},
		{name: "voucher excludes brand", rule: testdata.Sale25Voucher, product: testdata.NikeRunningShoes},
		{name: "voucher allows category", rule: testdata.TShirt15Voucher, product: testdata.PumaTShirt, want: true},
		{name: "voucher restricts category", rule: testdata.TShirt15Voucher, product: testdata.RoadsterJeans},
		{
			name:    "exclusion wins over allowed category",
			rule:    discount.Voucher{ExcludedBrands: []string{"PUMA"}, AllowedCategories: []string{"T-shirts"}},
			product: testdata.PumaTShirt,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rule.AppliesToProduct(tt.product); got != tt.want {
				t.Errorf("AppliesToProduct = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBankOfferAppliesToPayment(t *testing.T) {
	creditOnly := testdata.ICICIBankOffer
	creditOnly.CardType = "CREDIT"

	tests := []struct {
		name    string
		offer   discount.BankOffer
		payment *models.PaymentInfo
		want    bool
	}{
		{name: "matching bank card", offer: testdata.ICICIBankOffer, payment: testdata.ICICICreditCard, want: true},
		{name: "other bank", offer: testdata.ICICIBankOffer, payment: testdata.HDFCCreditCard},
		{name: "same bank, wrong method", offer: testdata.ICICIBankOffer, payment: testdata.ICICIUPI},
		{name: "nil payment", offer: testdata.ICICIBankOffer},
		{name: "missing bank name", offer: testdata.ICICIBankOffer, payment: &models.PaymentInfo{Method: "CARD"}},
		{name: "card type matches ignoring case", offer: creditOnly, payment: &models.PaymentInfo{Method: "card", BankName: ptr("icici"), CardType: ptr("credit")}, want: true},
		{name: "card type differs", offer: creditOnly, payment: &models.PaymentInfo{Method: "CARD", BankName: ptr("ICICI"), CardType: ptr("DEBIT")}},
		{name: "card type required but unknown", offer: creditOnly, payment: &models.PaymentInfo{Method: "CARD", BankName: ptr("ICICI")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.offer.AppliesToPayment(tt.payment); got != tt.want {
				t.Errorf("AppliesToPayment = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBest(t *testing.T) {
	flat := discount.BrandDiscount{Offer: discount.Offer{ID: "flat", Percentage: dec("30")}, Brand: "PUMA"}
	capped := discount.BrandDiscount{Offer: discount.Offer{ID: "capped", Percentage: dec("50"), MaxAmount: ptr(dec("100"))}, Brand: "PUMA"}
	other := discount.BrandDiscount{Offer: discount.Offer{ID: "other", Percentage: dec("90")}, Brand: "Nike"}
	rules := []discount.BrandDiscount{capped, flat, other}

	applies := func(d discount.BrandDiscount) bool { return d.AppliesToProduct(testdata.PumaTShirt) }

	t.Run("picks largest amount, not largest percentage", func(t *testing.T) {
		best, amount, ok := discount.Best(rules, dec("1000"), applies)
		if !ok || best.ID != "flat" || !amount.Equal(dec("300")) {
			t.Errorf("Best = (%s, %s, %v), want (flat, 300, true)", best.ID, amount, ok)
		}
	})

	t.Run("cap stops mattering on small prices", func(t *testing.T) {
		best, amount, ok := discount.Best(rules, dec("100"), applies)
		if !ok || best.ID != "capped" || !amount.Equal(dec("50")) {
			t.Errorf("Best = (%s, %s, %v), want (capped, 50, true)", best.ID, amount, ok)
		}
	})

	t.Run("no applicable rule", func(t *testing.T) {
		if _, _, ok := discount.Best(rules, dec("100"), func(discount.BrandDiscount) bool { return false }); ok {
			t.Error("ok = true, want false")
		}
	})
}
