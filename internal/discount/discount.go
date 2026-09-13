// Package discount defines the discount rule types offered by the storefront.
//
// Each rule type embeds Offer, which carries the terms common to every
// discount (percentage, cap, validity window), and adds its own targeting
// logic. Rules that target products implement ProductRule so the pricing code
// can treat brand, category and future product-level discounts uniformly.
package discount

import (
	"slices"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/ransh7/unifize-assignment/internal/models"
)

// MoneyPlaces is the number of decimal places monetary amounts are rounded to.
const MoneyPlaces = 2

var hundred = decimal.NewFromInt(100)

// Offer holds the terms shared by every discount rule.
type Offer struct {
	ID   string `json:"id"`
	Name string `json:"name"` // Customer-facing; used as the key in DiscountedPrice.AppliedDiscounts.

	// Percentage is the discount rate, where 10 means 10%.
	Percentage decimal.Decimal `json:"percentage"`
	// MaxAmount optionally caps the absolute discount.
	MaxAmount *decimal.Decimal `json:"max_amount,omitempty"`

	// ValidFrom and ValidUntil bound when the offer is active. A zero value
	// leaves that side of the window open.
	ValidFrom  time.Time `json:"valid_from"`
	ValidUntil time.Time `json:"valid_until"`
}

// Terms returns the offer itself. Because Offer is embedded in every rule
// type, Terms gives generic code access to the common terms of any rule.
func (o Offer) Terms() Offer { return o }

// NotYetActive reports whether t is before the start of the validity window.
func (o Offer) NotYetActive(t time.Time) bool {
	return !o.ValidFrom.IsZero() && t.Before(o.ValidFrom)
}

// Expired reports whether t is after the end of the validity window.
func (o Offer) Expired(t time.Time) bool {
	return !o.ValidUntil.IsZero() && t.After(o.ValidUntil)
}

// IsActiveAt reports whether the validity window contains t.
func (o Offer) IsActiveAt(t time.Time) bool {
	return !o.NotYetActive(t) && !o.Expired(t)
}

// AmountOff returns the discount this offer gives on price, rounded to
// MoneyPlaces and limited by MaxAmount. The result is never more than price.
func (o Offer) AmountOff(price decimal.Decimal) decimal.Decimal {
	amount := price.Mul(o.Percentage).Div(hundred).Round(MoneyPlaces)
	if o.MaxAmount != nil && amount.GreaterThan(*o.MaxAmount) {
		amount = *o.MaxAmount
	}
	return decimal.Min(amount, price)
}

// Rule is implemented by every discount rule type.
type Rule interface {
	Terms() Offer
}

// ProductRule is a discount that targets individual products.
type ProductRule interface {
	Rule
	AppliesToProduct(p models.Product) bool
}

// BrandDiscount applies to every product of a brand, across categories.
// Example: "Min 40% off on PUMA".
type BrandDiscount struct {
	Offer
	Brand string `json:"brand"`
}

// AppliesToProduct implements ProductRule.
func (d BrandDiscount) AppliesToProduct(p models.Product) bool {
	return strings.EqualFold(d.Brand, p.Brand)
}

// CategoryDiscount applies to every product in a category.
// Example: "Extra 10% off on T-shirts".
type CategoryDiscount struct {
	Offer
	Category string `json:"category"`
}

// AppliesToProduct implements ProductRule.
func (d CategoryDiscount) AppliesToProduct(p models.Product) bool {
	return strings.EqualFold(d.Category, p.Category)
}

// Voucher is a discount unlocked by entering a code.
// Example: "SUPER69" for 69% off on any product.
type Voucher struct {
	Offer
	Code string `json:"code"`

	// ExcludedBrands lists brands the voucher never discounts.
	ExcludedBrands []string `json:"excluded_brands,omitempty"`
	// AllowedCategories restricts the voucher to these categories. Empty
	// means every category.
	AllowedCategories []string `json:"allowed_categories,omitempty"`
	// MinCustomerTier is the lowest customer tier allowed to use the
	// voucher. Empty means any tier.
	MinCustomerTier string `json:"min_customer_tier,omitempty"`
}

// AppliesToProduct implements ProductRule using the voucher's brand exclusions
// and category restrictions.
func (v Voucher) AppliesToProduct(p models.Product) bool {
	if containsFold(v.ExcludedBrands, p.Brand) {
		return false
	}
	return len(v.AllowedCategories) == 0 || containsFold(v.AllowedCategories, p.Category)
}

// BankOffer is an instant discount for paying with a particular bank.
// Example: "10% instant discount on ICICI Bank cards".
type BankOffer struct {
	Offer
	BankName string `json:"bank_name"`
	// PaymentMethod optionally restricts the offer, e.g. to CARD.
	PaymentMethod string `json:"payment_method,omitempty"`
	// CardType optionally restricts the offer, e.g. to CREDIT.
	CardType string `json:"card_type,omitempty"`
}

// AppliesToPayment reports whether the offer can be used with p.
func (b BankOffer) AppliesToPayment(p *models.PaymentInfo) bool {
	if p == nil {
		return false
	}
	return matchesOptional(b.PaymentMethod, &p.Method) &&
		matchesOptional(b.BankName, p.BankName) &&
		matchesOptional(b.CardType, p.CardType)
}

// NormalizeCode returns the canonical form of a voucher code.
func NormalizeCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

// Best returns the rule among those satisfying applies that gives the largest
// discount on price, together with that amount. It reports false if no rule
// applies.
func Best[R Rule](rules []R, price decimal.Decimal, applies func(R) bool) (best R, amount decimal.Decimal, ok bool) {
	for _, r := range rules {
		if !applies(r) {
			continue
		}
		a := r.Terms().AmountOff(price)
		if !ok || a.GreaterThan(amount) {
			best, amount, ok = r, a, true
		}
	}
	return best, amount, ok
}

// matchesOptional reports whether actual satisfies want. An empty want matches
// anything; otherwise actual must be present and equal ignoring case.
func matchesOptional(want string, actual *string) bool {
	if want == "" {
		return true
	}
	return actual != nil && strings.EqualFold(want, *actual)
}

func containsFold(values []string, s string) bool {
	return slices.ContainsFunc(values, func(v string) bool { return strings.EqualFold(v, s) })
}
