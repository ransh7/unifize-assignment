// Package models defines the domain types shared across the discount service:
// the catalogue/cart types supplied by the assignment plus the discount rule
// definitions the service evaluates.
package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// BrandTier classifies a brand by its market positioning.
type BrandTier string

// Supported brand tiers.
const (
	BrandTierPremium BrandTier = "premium"
	BrandTierRegular BrandTier = "regular"
	BrandTierBudget  BrandTier = "budget"
)

// Product is a sellable catalogue item.
type Product struct {
	ID           string          `json:"id"`
	Brand        string          `json:"brand"`
	BrandTier    BrandTier       `json:"brand_tier"`
	Category     string          `json:"category"`
	BasePrice    decimal.Decimal `json:"base_price"`
	CurrentPrice decimal.Decimal `json:"current_price"` // After brand/category discount
}

// CartItem is a product placed in the cart with a quantity and size.
type CartItem struct {
	Product  Product `json:"product"`
	Quantity int     `json:"quantity"`
	Size     string  `json:"size"`
}

// Payment methods understood by the service.
const (
	PaymentMethodCard = "CARD"
	PaymentMethodUPI  = "UPI"
)

// PaymentInfo describes how the customer intends to pay.
type PaymentInfo struct {
	Method   string  `json:"method"`    // CARD, UPI, etc
	BankName *string `json:"bank_name"` // Optional
	CardType *string `json:"card_type"` // Optional: CREDIT, DEBIT
}

// DiscountedPrice is the result of pricing a cart.
type DiscountedPrice struct {
	OriginalPrice    decimal.Decimal            `json:"original_price"`
	FinalPrice       decimal.Decimal            `json:"final_price"`
	AppliedDiscounts map[string]decimal.Decimal `json:"applied_discounts"` // discount_name -> amount
	Message          string                     `json:"message"`
}

// Customer tiers, ordered from lowest to highest.
const (
	CustomerTierRegular  = "regular"
	CustomerTierSilver   = "silver"
	CustomerTierGold     = "gold"
	CustomerTierPlatinum = "platinum"
)

var customerTierRank = map[string]int{
	CustomerTierRegular:  0,
	CustomerTierSilver:   1,
	CustomerTierGold:     2,
	CustomerTierPlatinum: 3,
}

// CustomerProfile holds the customer attributes relevant to discount eligibility.
type CustomerProfile struct {
	ID   string `json:"id"`
	Tier string `json:"tier"`
}

// MeetsTier reports whether the customer's tier is at least minTier.
// An empty minTier imposes no requirement; an unknown customer tier never
// satisfies a non-empty requirement.
func (c CustomerProfile) MeetsTier(minTier string) bool {
	if minTier == "" {
		return true
	}
	have, ok := customerTierRank[c.Tier]
	if !ok {
		return false
	}
	want, ok := customerTierRank[minTier]
	if !ok {
		return false
	}
	return have >= want
}

// DiscountType identifies which pricing stage a discount belongs to.
type DiscountType string

// Supported discount types.
const (
	DiscountTypeBrand     DiscountType = "brand"
	DiscountTypeCategory  DiscountType = "category"
	DiscountTypeVoucher   DiscountType = "voucher"
	DiscountTypeBankOffer DiscountType = "bank_offer"
)

// Discount is a percentage-based discount rule. Which fields are meaningful
// depends on Type:
//   - brand:      Brand
//   - category:   Category
//   - voucher:    Code, ExcludedBrands, AllowedCategories, MinCustomerTier
//   - bank_offer: BankName, PaymentMethod, CardType
type Discount struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Type       DiscountType    `json:"type"`
	Percentage decimal.Decimal `json:"percentage"` // 10 means 10%

	// MaxDiscountAmount optionally caps the absolute discount.
	MaxDiscountAmount *decimal.Decimal `json:"max_discount_amount,omitempty"`

	// ValidFrom and ValidUntil bound when the discount is active. A zero
	// value means the bound is open.
	ValidFrom  time.Time `json:"valid_from"`
	ValidUntil time.Time `json:"valid_until"`

	Brand    string `json:"brand,omitempty"`
	Category string `json:"category,omitempty"`

	Code              string   `json:"code,omitempty"`
	ExcludedBrands    []string `json:"excluded_brands,omitempty"`
	AllowedCategories []string `json:"allowed_categories,omitempty"`
	MinCustomerTier   string   `json:"min_customer_tier,omitempty"`

	BankName      string `json:"bank_name,omitempty"`
	PaymentMethod string `json:"payment_method,omitempty"`
	CardType      string `json:"card_type,omitempty"`
}

// IsActiveAt reports whether the discount's validity window contains t.
func (d Discount) IsActiveAt(t time.Time) bool {
	if !d.ValidFrom.IsZero() && t.Before(d.ValidFrom) {
		return false
	}
	if !d.ValidUntil.IsZero() && t.After(d.ValidUntil) {
		return false
	}
	return true
}
