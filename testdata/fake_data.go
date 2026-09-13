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

	"github.com/ransh7/unifize-assignment/internal/discount"
	"github.com/ransh7/unifize-assignment/internal/models"
	"github.com/ransh7/unifize-assignment/internal/repository"
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
	PumaBrandDiscount = discount.BrandDiscount{
		Offer: discount.Offer{
			ID:         "disc-brand-puma",
			Name:       "Min 40% off on PUMA",
			Percentage: decimal.NewFromInt(40),
		},
		Brand: "PUMA",
	}

	TShirtCategoryDiscount = discount.CategoryDiscount{
		Offer: discount.Offer{
			ID:         "disc-category-tshirts",
			Name:       "Extra 10% off on T-shirts",
			Percentage: decimal.NewFromInt(10),
		},
		Category: "T-shirts",
	}

	ICICIBankOffer = discount.BankOffer{
		Offer: discount.Offer{
			ID:         "disc-bank-icici",
			Name:       "10% instant discount on ICICI Bank cards",
			Percentage: decimal.NewFromInt(10),
		},
		BankName:      "ICICI",
		PaymentMethod: models.PaymentMethodCard,
	}
)

// Vouchers.
var (
	Super69Voucher = discount.Voucher{
		Offer: discount.Offer{
			ID:         "voucher-super69",
			Name:       "SUPER69: 69% off",
			Percentage: decimal.NewFromInt(69),
		},
		Code: "SUPER69",
	}

	Gold20Voucher = discount.Voucher{
		Offer: discount.Offer{
			ID:         "voucher-gold20",
			Name:       "GOLD20: 20% off for gold members",
			Percentage: decimal.NewFromInt(20),
		},
		Code:            "GOLD20",
		MinCustomerTier: models.CustomerTierGold,
	}

	TShirt15Voucher = discount.Voucher{
		Offer: discount.Offer{
			ID:         "voucher-tshirt15",
			Name:       "TSHIRT15: 15% off T-shirts",
			Percentage: decimal.NewFromInt(15),
		},
		Code:              "TSHIRT15",
		AllowedCategories: []string{"T-shirts"},
	}

	Sale25Voucher = discount.Voucher{
		Offer: discount.Offer{
			ID:         "voucher-sale25",
			Name:       "SALE25: 25% off, excludes Nike",
			Percentage: decimal.NewFromInt(25),
		},
		Code:           "SALE25",
		ExcludedBrands: []string{"Nike"},
	}

	Expired10Voucher = discount.Voucher{
		Offer: discount.Offer{
			ID:         "voucher-expired10",
			Name:       "EXPIRED10: 10% off",
			Percentage: decimal.NewFromInt(10),
			ValidUntil: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
		Code: "EXPIRED10",
	}

	Launch30Voucher = discount.Voucher{
		Offer: discount.Offer{
			ID:         "voucher-launch30",
			Name:       "LAUNCH30: 30% off",
			Percentage: decimal.NewFromInt(30),
			ValidFrom:  time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
		Code: "LAUNCH30",
	}
)

// Rules returns every fake discount rule, ready to load into a repository.
func Rules() repository.Rules {
	return repository.Rules{
		Brands:     []discount.BrandDiscount{PumaBrandDiscount},
		Categories: []discount.CategoryDiscount{TShirtCategoryDiscount},
		Vouchers: []discount.Voucher{
			Super69Voucher, Gold20Voucher, TShirt15Voucher, Sale25Voucher, Expired10Voucher, Launch30Voucher,
		},
		BankOffers: []discount.BankOffer{ICICIBankOffer},
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
