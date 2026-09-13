// Package testdata contains fake catalogue, discount, customer and payment
// data used by tests and the demo command.
//
// The primary scenario is a PUMA T-shirt that qualifies for:
//   - "Min 40% off on PUMA" (brand discount)
//   - "Extra 10% off on T-shirts" (category discount)
//   - "10% instant discount on ICICI Bank cards" (bank offer)
package testdata

import (
	"time"

	"github.com/shopspring/decimal"

	"github.com/ransh7/unifize-assignment/internal/models"
)

// Now is the fixed reference time that the fake discount validity windows are
// defined against.
var Now = time.Date(2026, time.September, 13, 12, 0, 0, 0, time.UTC)

// Products.
var (
	PumaTShirt = models.Product{
		ID:           "prod-puma-tshirt-001",
		Brand:        "PUMA",
		BrandTier:    models.BrandTierRegular,
		Category:     "T-shirts",
		BasePrice:    decimal.NewFromInt(1000),
		CurrentPrice: decimal.NewFromInt(540), // 1000 -40% -10%
	}

	NikeRunningShoes = models.Product{
		ID:           "prod-nike-shoes-001",
		Brand:        "Nike",
		BrandTier:    models.BrandTierPremium,
		Category:     "Shoes",
		BasePrice:    decimal.NewFromInt(5000),
		CurrentPrice: decimal.NewFromInt(5000),
	}

	RoadsterJeans = models.Product{
		ID:           "prod-roadster-jeans-001",
		Brand:        "Roadster",
		BrandTier:    models.BrandTierBudget,
		Category:     "Jeans",
		BasePrice:    decimal.NewFromInt(1500),
		CurrentPrice: decimal.NewFromInt(1500),
	}
)

// Discounts for the multiple discount scenario.
var (
	PumaBrandDiscount = models.Discount{
		ID:         "disc-brand-puma",
		Name:       "Min 40% off on PUMA",
		Type:       models.DiscountTypeBrand,
		Percentage: decimal.NewFromInt(40),
		Brand:      "PUMA",
	}

	TShirtCategoryDiscount = models.Discount{
		ID:         "disc-category-tshirts",
		Name:       "Extra 10% off on T-shirts",
		Type:       models.DiscountTypeCategory,
		Percentage: decimal.NewFromInt(10),
		Category:   "T-shirts",
	}

	ICICIBankOffer = models.Discount{
		ID:            "disc-bank-icici",
		Name:          "10% instant discount on ICICI Bank cards",
		Type:          models.DiscountTypeBankOffer,
		Percentage:    decimal.NewFromInt(10),
		BankName:      "ICICI",
		PaymentMethod: models.PaymentMethodCard,
	}
)

// Vouchers.
var (
	Super69Voucher = models.Discount{
		ID:         "voucher-super69",
		Name:       "SUPER69: 69% off",
		Type:       models.DiscountTypeVoucher,
		Percentage: decimal.NewFromInt(69),
		Code:       "SUPER69",
	}

	Gold20Voucher = models.Discount{
		ID:              "voucher-gold20",
		Name:            "GOLD20: 20% off for gold members",
		Type:            models.DiscountTypeVoucher,
		Percentage:      decimal.NewFromInt(20),
		Code:            "GOLD20",
		MinCustomerTier: models.CustomerTierGold,
	}

	TShirt15Voucher = models.Discount{
		ID:                "voucher-tshirt15",
		Name:              "TSHIRT15: 15% off T-shirts",
		Type:              models.DiscountTypeVoucher,
		Percentage:        decimal.NewFromInt(15),
		Code:              "TSHIRT15",
		AllowedCategories: []string{"T-shirts"},
	}

	Sale25Voucher = models.Discount{
		ID:             "voucher-sale25",
		Name:           "SALE25: 25% off, excludes Nike",
		Type:           models.DiscountTypeVoucher,
		Percentage:     decimal.NewFromInt(25),
		Code:           "SALE25",
		ExcludedBrands: []string{"Nike"},
	}

	Expired10Voucher = models.Discount{
		ID:         "voucher-expired10",
		Name:       "EXPIRED10: 10% off",
		Type:       models.DiscountTypeVoucher,
		Percentage: decimal.NewFromInt(10),
		Code:       "EXPIRED10",
		ValidUntil: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
	}
)

// AllDiscounts returns every fake discount rule.
func AllDiscounts() []models.Discount {
	return []models.Discount{
		PumaBrandDiscount,
		TShirtCategoryDiscount,
		ICICIBankOffer,
		Super69Voucher,
		Gold20Voucher,
		TShirt15Voucher,
		Sale25Voucher,
		Expired10Voucher,
	}
}

// Customers.
var (
	RegularCustomer = models.CustomerProfile{ID: "cust-001", Tier: models.CustomerTierRegular}
	GoldCustomer    = models.CustomerProfile{ID: "cust-002", Tier: models.CustomerTierGold}
)

// Payments.
var (
	ICICICreditCard = &models.PaymentInfo{
		Method:   models.PaymentMethodCard,
		BankName: ptr("ICICI"),
		CardType: ptr("CREDIT"),
	}

	HDFCCreditCard = &models.PaymentInfo{
		Method:   models.PaymentMethodCard,
		BankName: ptr("HDFC"),
		CardType: ptr("CREDIT"),
	}

	ICICIUPI = &models.PaymentInfo{
		Method:   models.PaymentMethodUPI,
		BankName: ptr("ICICI"),
	}
)

func ptr[T any](v T) *T { return &v }
