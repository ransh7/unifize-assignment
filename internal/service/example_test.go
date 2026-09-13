package service_test

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/ransh7/unifize-assignment/internal/models"
	"github.com/ransh7/unifize-assignment/internal/repository"
	"github.com/ransh7/unifize-assignment/internal/service"
	"github.com/ransh7/unifize-assignment/testdata"
)

func exampleService() service.DiscountService {
	repo, err := repository.NewInMemoryRepository(testdata.Rules())
	if err != nil {
		panic(err)
	}
	return service.NewDiscountService(repo, service.WithClock(func() time.Time { return testdata.Now }))
}

// The multiple discount scenario: a PUMA T-shirt gets the brand discount, then
// the T-shirt category discount, then the ICICI bank offer, each applied to
// the price left by the previous one.
func ExampleDiscountService_CalculateCartDiscounts() {
	svc := exampleService()
	cart := []models.CartItem{{Product: testdata.PumaTShirt, Quantity: 1, Size: "M"}}

	result, err := svc.CalculateCartDiscounts(context.Background(), cart, testdata.RegularCustomer, testdata.ICICICreditCard)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("original:", result.OriginalPrice.StringFixed(2))
	for _, name := range slices.Sorted(maps.Keys(result.AppliedDiscounts)) {
		fmt.Printf("  %s: -%s\n", name, result.AppliedDiscounts[name].StringFixed(2))
	}
	fmt.Println("final:", result.FinalPrice.StringFixed(2))
	fmt.Println(result.Message)
	// Output:
	// original: 1000.00
	//   10% instant discount on ICICI Bank cards: -54.00
	//   Extra 10% off on T-shirts: -60.00
	//   Min 40% off on PUMA: -400.00
	// final: 486.00
	// Applied 3 discount(s): you save 514.00 (51.40%)
}

// A voucher code is passed to CalculateCartDiscounts through the context.
func ExampleWithVoucherCode() {
	svc := exampleService()
	cart := []models.CartItem{{Product: testdata.PumaTShirt, Quantity: 1, Size: "M"}}

	ctx := service.WithVoucherCode(context.Background(), "SUPER69")
	result, err := svc.CalculateCartDiscounts(ctx, cart, testdata.RegularCustomer, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(result.FinalPrice.StringFixed(2))
	// Output:
	// 167.40
}

// Validation failures carry a sentinel reason for programmatic handling and a
// detail for display.
func ExampleDiscountService_ValidateDiscountCode() {
	svc := exampleService()
	cart := []models.CartItem{{Product: testdata.NikeRunningShoes, Quantity: 1, Size: "9"}}

	ok, err := svc.ValidateDiscountCode(context.Background(), "tshirt15", cart, testdata.RegularCustomer)
	fmt.Println(ok)
	fmt.Println(errors.Is(err, service.ErrNoEligibleItems))
	fmt.Println(err)
	// Output:
	// false
	// true
	// discount code "TSHIRT15": no eligible items in cart: only valid on categories: T-shirts
}
