// Package pricing provides a small pipeline for pricing a cart in ordered
// stages. Each Stage reduces prices on a shared Cart, and later stages see the
// prices left by earlier ones, which is what makes discounts compound.
package pricing

import (
	"context"
	"iter"
	"time"

	"github.com/shopspring/decimal"

	"github.com/ransh7/unifize-assignment/internal/models"
)

// Line is a cart item together with its running price.
type Line struct {
	Item models.CartItem
	// Price is the price of the whole line (unit price x quantity) after the
	// discounts applied so far.
	Price decimal.Decimal
}

// Cart is the mutable pricing state passed through a Pipeline.
type Cart struct {
	Lines       []Line
	Customer    models.CustomerProfile
	Payment     *models.PaymentInfo // nil when payment is unknown
	VoucherCode string              // empty when no voucher was entered
	Now         time.Time           // time at which offer validity is evaluated

	original     decimal.Decimal
	cartDiscount decimal.Decimal
	applied      map[string]decimal.Decimal
}

// NewCart returns a Cart whose lines start at base price x quantity.
func NewCart(items []models.CartItem, customer models.CustomerProfile, payment *models.PaymentInfo, now time.Time) *Cart {
	c := &Cart{
		Lines:    make([]Line, len(items)),
		Customer: customer,
		Payment:  payment,
		Now:      now,
		applied:  make(map[string]decimal.Decimal),
	}
	for i, item := range items {
		gross := item.Product.BasePrice.Mul(decimal.NewFromInt(int64(item.Quantity)))
		c.Lines[i] = Line{Item: item, Price: gross}
		c.original = c.original.Add(gross)
	}
	return c
}

// Original returns the cart price before any discount.
func (c *Cart) Original() decimal.Decimal { return c.original }

// Total returns the current cart price: the sum of line prices less any
// cart-level discounts.
func (c *Cart) Total() decimal.Decimal {
	total := decimal.Zero
	for i := range c.Lines {
		total = total.Add(c.Lines[i].Price)
	}
	return total.Sub(c.cartDiscount)
}

// Products yields the product of every line.
func (c *Cart) Products() iter.Seq[models.Product] {
	return func(yield func(models.Product) bool) {
		for i := range c.Lines {
			if !yield(c.Lines[i].Item.Product) {
				return
			}
		}
	}
}

// DiscountLine reduces the price of line i by amount and records it under
// name. Non-positive amounts are ignored.
func (c *Cart) DiscountLine(i int, name string, amount decimal.Decimal) {
	if !amount.IsPositive() {
		return
	}
	c.Lines[i].Price = c.Lines[i].Price.Sub(amount)
	c.record(name, amount)
}

// DiscountTotal reduces the cart total by amount without attributing it to a
// line, and records it under name. Non-positive amounts are ignored.
//
// Because line prices are not reduced, stages that call DiscountLine must run
// before stages that call DiscountTotal.
func (c *Cart) DiscountTotal(name string, amount decimal.Decimal) {
	if !amount.IsPositive() {
		return
	}
	c.cartDiscount = c.cartDiscount.Add(amount)
	c.record(name, amount)
}

// Applied returns a copy of the discounts applied so far, keyed by name.
func (c *Cart) Applied() map[string]decimal.Decimal {
	out := make(map[string]decimal.Decimal, len(c.applied))
	for k, v := range c.applied {
		out[k] = v
	}
	return out
}

func (c *Cart) record(name string, amount decimal.Decimal) {
	c.applied[name] = c.applied[name].Add(amount)
}

// Stage is one step of the pricing pipeline. Implement Stage to add a new kind
// of discount; stages that discount individual lines must be placed before
// stages that discount the cart total.
type Stage interface {
	Apply(ctx context.Context, cart *Cart) error
}

// Pipeline runs stages in order.
type Pipeline struct {
	stages []Stage
}

// NewPipeline returns a Pipeline running stages in the given order.
func NewPipeline(stages ...Stage) Pipeline {
	return Pipeline{stages: stages}
}

// Run applies every stage to cart, stopping at the first error or when ctx is
// done.
func (p Pipeline) Run(ctx context.Context, cart *Cart) error {
	for _, stage := range p.stages {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := stage.Apply(ctx, cart); err != nil {
			return err
		}
	}
	return nil
}
