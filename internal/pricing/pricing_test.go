package pricing_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/ransh7/unifize-assignment/internal/models"
	"github.com/ransh7/unifize-assignment/internal/pricing"
	"github.com/ransh7/unifize-assignment/testdata"
)

// stageFunc adapts a function to pricing.Stage for tests.
type stageFunc func(ctx context.Context, cart *pricing.Cart) error

func (f stageFunc) Apply(ctx context.Context, cart *pricing.Cart) error { return f(ctx, cart) }

func newCart() *pricing.Cart {
	items := []models.CartItem{
		{Product: testdata.PumaTShirt, Quantity: 2},
		{Product: testdata.RoadsterJeans, Quantity: 1},
	}
	return pricing.NewCart(items, testdata.RegularCustomer, nil, testdata.Now)
}

func TestCartLedger(t *testing.T) {
	cart := newCart()

	if want := decimal.NewFromInt(3500); !cart.Original().Equal(want) || !cart.Total().Equal(want) {
		t.Fatalf("Original, Total = %s, %s; want %s", cart.Original(), cart.Total(), want)
	}

	cart.DiscountLine(0, "line", decimal.NewFromInt(200))
	cart.DiscountLine(1, "line", decimal.NewFromInt(100))
	cart.DiscountTotal("cart", decimal.NewFromInt(50))
	cart.DiscountTotal("ignored", decimal.NewFromInt(-5))
	cart.DiscountLine(0, "ignored", decimal.Zero)

	if want := decimal.NewFromInt(3150); !cart.Total().Equal(want) {
		t.Errorf("Total = %s, want %s", cart.Total(), want)
	}
	if !cart.Original().Equal(decimal.NewFromInt(3500)) {
		t.Errorf("Original changed to %s", cart.Original())
	}

	applied := cart.Applied()
	if len(applied) != 2 || !applied["line"].Equal(decimal.NewFromInt(300)) || !applied["cart"].Equal(decimal.NewFromInt(50)) {
		t.Errorf("Applied = %v, want line:300 cart:50", applied)
	}

	applied["line"] = decimal.Zero
	if !cart.Applied()["line"].Equal(decimal.NewFromInt(300)) {
		t.Error("mutating the result of Applied changed the cart")
	}
}

func TestPipelineRun(t *testing.T) {
	var order []string
	record := func(name string) pricing.Stage {
		return stageFunc(func(context.Context, *pricing.Cart) error {
			order = append(order, name)
			return nil
		})
	}
	errStage := errors.New("stage failed")

	t.Run("runs stages in order", func(t *testing.T) {
		order = nil
		err := pricing.NewPipeline(record("a"), record("b"), record("c")).Run(context.Background(), newCart())
		if err != nil || !slices.Equal(order, []string{"a", "b", "c"}) {
			t.Errorf("Run = %v, order %v; want nil, [a b c]", err, order)
		}
	})

	t.Run("stops at first error", func(t *testing.T) {
		order = nil
		failing := stageFunc(func(context.Context, *pricing.Cart) error { return errStage })
		err := pricing.NewPipeline(record("a"), failing, record("c")).Run(context.Background(), newCart())
		if !errors.Is(err, errStage) || !slices.Equal(order, []string{"a"}) {
			t.Errorf("Run = %v, order %v; want %v, [a]", err, order, errStage)
		}
	})

	t.Run("stops when context is cancelled", func(t *testing.T) {
		order = nil
		ctx, cancel := context.WithCancel(context.Background())
		cancelling := stageFunc(func(context.Context, *pricing.Cart) error {
			cancel()
			return nil
		})
		err := pricing.NewPipeline(cancelling, record("b")).Run(ctx, newCart())
		if !errors.Is(err, context.Canceled) || len(order) != 0 {
			t.Errorf("Run = %v, order %v; want context.Canceled, []", err, order)
		}
	})
}
