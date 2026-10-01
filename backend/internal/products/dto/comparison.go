package dto

type ProductComparison struct {
	Products       []ProductDetail       `json:"products"`
	Recommendation ProductRecommendation `json:"recommendation"`
}

type ProductRecommendation struct {
	WinnerIDs     []string                 `json:"winnerIds"`
	IsTie         bool                     `json:"isTie"`
	Score         int                      `json:"score"`
	TotalCriteria int                      `json:"totalCriteria"`
	Scores        []ProductComparisonScore `json:"scores"`
}

type ProductComparisonScore struct {
	ProductID string   `json:"productId"`
	Score     int      `json:"score"`
	Reasons   []string `json:"reasons"`
}
