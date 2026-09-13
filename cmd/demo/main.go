// Command demo prices the sample "multiple discount" cart and prints the result
// as JSON.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/ransh7/unifize-assignment/internal/models"
	"github.com/ransh7/unifize-assignment/internal/repository"
	"github.com/ransh7/unifize-assignment/internal/service"
	"github.com/ransh7/unifize-assignment/testdata"
)

func main() {
	voucher := flag.String("voucher", "", "optional voucher code to apply, e.g. SUPER69")
	flag.Parse()

	repo, err := repository.NewInMemoryRepository(testdata.Rules())
	if err != nil {
		log.Fatalf("loading discount rules: %v", err)
	}
	svc := service.NewDiscountService(repo)

	ctx := context.Background()
	if *voucher != "" {
		ctx = service.WithVoucherCode(ctx, *voucher)
	}

	cart := []models.CartItem{{Product: testdata.PumaTShirt, Quantity: 1, Size: "M"}}

	result, err := svc.CalculateCartDiscounts(ctx, cart, testdata.RegularCustomer, testdata.ICICICreditCard)
	if err != nil {
		log.Fatalf("calculating discounts: %v", err)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "encoding result: %v\n", err)
		os.Exit(1)
	}
}
