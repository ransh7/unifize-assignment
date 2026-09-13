package service

import (
	"context"
	"fmt"
	"iter"
	"time"

	"github.com/shopspring/decimal"

	"github.com/ransh7/unifize-assignment/internal/discount"
	"github.com/ransh7/unifize-assignment/internal/models"
	"github.com/ransh7/unifize-assignment/internal/pricing"
	"github.com/ransh7/unifize-assignment/internal/repository"
)

// productDiscountStage applies, to each line, the most valuable active rule
// of type R that targets the line's product. It serves brand and category
// discounts, and any future ProductRule type.
type productDiscountStage[R discount.ProductRule] struct {
	kind string // used in error messages, e.g. "brand"
	load func(ctx context.Context, products iter.Seq[models.Product], t time.Time) ([]R, error)
}

// Apply implements pricing.Stage.
func (s productDiscountStage[R]) Apply(ctx context.Context, cart *pricing.Cart) error {
	rules, err := s.load(ctx, cart.Products(), cart.Now)
	if err != nil {
		return fmt.Errorf("loading %s discounts: %w", s.kind, err)
	}
	if len(rules) == 0 {
		return nil
	}
	for i := range cart.Lines {
		product := cart.Lines[i].Item.Product
		rule, amount, ok := discount.Best(rules, cart.Lines[i].Price, func(r R) bool {
			return r.AppliesToProduct(product)
		})
		if ok {
			cart.DiscountLine(i, rule.Terms().Name, amount)
		}
	}
	return nil
}

// voucherStage applies the voucher entered by the customer, if any, to the
// current price of the lines it is eligible for.
type voucherStage struct {
	vouchers voucherChecker
}

// Apply implements pricing.Stage.
func (s voucherStage) Apply(ctx context.Context, cart *pricing.Cart) error {
	if cart.VoucherCode == "" {
		return nil
	}
	voucher, err := s.vouchers.Check(ctx, cart.VoucherCode, cart.Products(), cart.Customer, cart.Now)
	if err != nil {
		return err
	}

	eligible := decimal.Zero
	for i := range cart.Lines {
		if voucher.AppliesToProduct(cart.Lines[i].Item.Product) {
			eligible = eligible.Add(cart.Lines[i].Price)
		}
	}
	cart.DiscountTotal(voucher.Name, voucher.AmountOff(eligible))
	return nil
}

// bankOfferStage applies the most valuable bank offer matching the payment
// method to the cart total.
type bankOfferStage struct {
	offers repository.BankOfferSource
}

// Apply implements pricing.Stage.
func (s bankOfferStage) Apply(ctx context.Context, cart *pricing.Cart) error {
	if cart.Payment == nil {
		return nil
	}
	offers, err := s.offers.BankOffers(ctx, cart.Now)
	if err != nil {
		return fmt.Errorf("loading bank offers: %w", err)
	}
	offer, amount, ok := discount.Best(offers, cart.Total(), func(o discount.BankOffer) bool {
		return o.AppliesToPayment(cart.Payment)
	})
	if ok {
		cart.DiscountTotal(offer.Name, amount)
	}
	return nil
}
