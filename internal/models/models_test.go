package models_test

import (
	"testing"

	"github.com/ransh7/unifize-assignment/internal/models"
)

func TestCustomerProfileMeetsTier(t *testing.T) {
	tests := []struct {
		customerTier string
		minTier      string
		want         bool
	}{
		{customerTier: "regular", minTier: "", want: true},
		{customerTier: "", minTier: "", want: true},
		{customerTier: "gold", minTier: "gold", want: true},
		{customerTier: "platinum", minTier: "silver", want: true},
		{customerTier: "silver", minTier: "gold", want: false},
		{customerTier: "Gold", minTier: "GOLD", want: true},
		{customerTier: "unknown", minTier: "regular", want: false},
		{customerTier: "gold", minTier: "unknown", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.customerTier+">="+tt.minTier, func(t *testing.T) {
			c := models.CustomerProfile{Tier: tt.customerTier}
			if got := c.MeetsTier(tt.minTier); got != tt.want {
				t.Errorf("MeetsTier(%q) with tier %q = %v, want %v", tt.minTier, tt.customerTier, got, tt.want)
			}
		})
	}
}
