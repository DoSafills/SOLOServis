package dto

type DuplicateCandidate struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Brand      string `json:"brand"`
	Model      string `json:"model"`
	Category   string `json:"category"`
	OfferCount int64  `json:"offerCount"`
	MatchType  string `json:"matchType"`
}
