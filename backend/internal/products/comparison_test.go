package products

import (
	"testing"
)

func TestCompareProductsSelectsBestScoringProduct(t *testing.T) {
	products := []ComparisonProduct{
		{
			ID:     "product-a",
			Rating: 4.5,
			Offers: []ComparisonOffer{
				{Price: 100, Available: true},
			},
		},
		{
			ID:     "product-b",
			Rating: 4.0,
			Offers: []ComparisonOffer{
				{Price: 200, Available: true},
				{Price: 220, Available: true},
			},
		},
	}

	comparison := CompareProducts(products)

	if len(comparison.WinnerIDs) != 1 || comparison.WinnerIDs[0] != "product-a" {
		t.Fatalf("expected product-a to win, got %v", comparison.WinnerIDs)
	}
	if comparison.Score != 2 || comparison.IsTie {
		t.Fatalf("expected a 2-point non-tie recommendation, got %+v", comparison)
	}
}

func TestCompareProductsReportsTiesAndMissingData(t *testing.T) {
	products := []ComparisonProduct{
		{ID: "product-a", Rating: 4.0, Offers: []ComparisonOffer{{Price: 100, Available: true}}},
		{ID: "product-b", Rating: 4.0, Offers: []ComparisonOffer{{Price: 100, Available: true}}},
	}

	comparison := CompareProducts(products)
	if !comparison.IsTie || len(comparison.WinnerIDs) != 2 {
		t.Fatalf("expected both products to tie, got %+v", comparison)
	}

	comparison = CompareProducts([]ComparisonProduct{{ID: "product-a"}, {ID: "product-b"}})
	if len(comparison.WinnerIDs) != 0 {
		t.Fatalf("expected no winner when comparison data is missing, got %v", comparison.WinnerIDs)
	}
}
