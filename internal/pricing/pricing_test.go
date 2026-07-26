package pricing

import "testing"

func TestMonthlyStorageCost_KnownRegion(t *testing.T) {
	cost := MonthlyStorageCost(1*bytesPerGiB, "us-east-1")
	if cost != 0.023 {
		t.Fatalf("expected 0.023 for 1 GiB in us-east-1, got %v", cost)
	}
}

func TestMonthlyStorageCost_UnknownRegionFallsBackToUSEast1(t *testing.T) {
	known := MonthlyStorageCost(10*bytesPerGiB, "us-east-1")
	unknown := MonthlyStorageCost(10*bytesPerGiB, "mars-central-1")
	if known != unknown {
		t.Fatalf("expected unknown region to fall back to us-east-1 pricing: known=%v unknown=%v", known, unknown)
	}
}

func TestMonthlyStorageCost_ZeroOrNegativeSize(t *testing.T) {
	if cost := MonthlyStorageCost(0, "us-east-1"); cost != 0 {
		t.Fatalf("expected 0 for zero size, got %v", cost)
	}
	if cost := MonthlyStorageCost(-5, "us-east-1"); cost != 0 {
		t.Fatalf("expected 0 for negative size, got %v", cost)
	}
}

func TestMonthlyStorageCost_ScalesLinearly(t *testing.T) {
	one := MonthlyStorageCost(1*bytesPerGiB, "eu-central-1")
	ten := MonthlyStorageCost(10*bytesPerGiB, "eu-central-1")
	if ten < one*9.99 || ten > one*10.01 {
		t.Fatalf("expected roughly linear scaling: 1GiB=%v 10GiB=%v", one, ten)
	}
}
