package products

type ComparisonProduct struct {
	ID     string
	Rating float64
	Offers []ComparisonOffer
}

type ComparisonOffer struct {
	Price     float64
	Available bool
}

type ComparisonScore struct {
	ProductID string
	Score     int
	Reasons   []string
}

type ComparisonResult struct {
	WinnerIDs     []string
	IsTie         bool
	Score         int
	TotalCriteria int
	Scores        []ComparisonScore
}

type comparisonMetric struct {
	label         string
	values        []float64
	valid         []bool
	lowerIsBetter bool
}

func CompareProducts(products []ComparisonProduct) ComparisonResult {
	prices := make([]float64, len(products))
	priceAvailable := make([]bool, len(products))
	storeCounts := make([]float64, len(products))
	storeCountAvailable := make([]bool, len(products))
	ratings := make([]float64, len(products))
	ratingAvailable := make([]bool, len(products))

	for index, product := range products {
		for _, offer := range product.Offers {
			if !offer.Available {
				continue
			}

			storeCounts[index]++
			if offer.Price <= 0 {
				continue
			}
			if !priceAvailable[index] || offer.Price < prices[index] {
				prices[index] = offer.Price
				priceAvailable[index] = true
			}
		}

		storeCountAvailable[index] = storeCounts[index] > 0
		ratings[index] = product.Rating
		ratingAvailable[index] = product.Rating > 0
	}

	metrics := []comparisonMetric{
		{label: "menor precio disponible", values: prices, valid: priceAvailable, lowerIsBetter: true},
		{label: "mayor valoración", values: ratings, valid: ratingAvailable},
		{label: "más tiendas disponibles", values: storeCounts, valid: storeCountAvailable},
	}

	scores := make([]ComparisonScore, len(products))
	for index, product := range products {
		scores[index] = ComparisonScore{
			ProductID: product.ID,
			Reasons:   []string{},
		}
	}

	for _, metric := range metrics {
		bestValue := 0.0
		hasBestValue := false
		for index, value := range metric.values {
			if !metric.valid[index] {
				continue
			}
			if !hasBestValue || (metric.lowerIsBetter && value < bestValue) || (!metric.lowerIsBetter && value > bestValue) {
				bestValue = value
				hasBestValue = true
			}
		}
		if !hasBestValue {
			continue
		}

		for index, value := range metric.values {
			if metric.valid[index] && value == bestValue {
				scores[index].Score++
				scores[index].Reasons = append(scores[index].Reasons, metric.label)
			}
		}
	}

	result := ComparisonResult{
		WinnerIDs:     []string{},
		TotalCriteria: len(metrics),
		Scores:        scores,
	}
	for _, score := range scores {
		if score.Score > result.Score {
			result.Score = score.Score
		}
	}
	if result.Score == 0 {
		return result
	}

	for _, score := range scores {
		if score.Score == result.Score {
			result.WinnerIDs = append(result.WinnerIDs, score.ProductID)
		}
	}
	result.IsTie = len(result.WinnerIDs) > 1

	return result
}
