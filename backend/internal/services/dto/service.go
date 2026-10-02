package dto

type ServiceDetail struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Provider         string            `json:"provider"`
	Category         string            `json:"category"`
	Subcategory      string            `json:"subcategory"`
	Description      string            `json:"description"`
	MonthlyPrice     float64           `json:"monthlyPrice"`
	InstallationCost *float64          `json:"installationCost"`
	ContractMonths   *int              `json:"contractMonths"`
	Rating           float64           `json:"rating"`
	ReviewCount      int64             `json:"reviewCount"`
	Specs            map[string]string `json:"specs"`
	Benefits         []string          `json:"benefits"`
	Coverage         string            `json:"coverage"`
	Image            string            `json:"image"`
	PriceHistory     []PricePoint      `json:"priceHistory"`
}

type PricePoint struct {
	Date  string  `json:"date"`
	Price float64 `json:"price"`
}
