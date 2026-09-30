package dto

import "github.com/DoSafills/SOLOServis/backend/internal/money"

// ServiceDetail is the element type of the GET /services response and the body
// of GET /services/{publicID}.
//
// The service itself carries no price: a service has one offer per provider, so
// the cheapest offer is surfaced as MonthlyPrice and the full set remains
// reachable through Offers.
type ServiceDetail struct {
	ID               string              `json:"id" format:"uuid" doc:"Public identifier of the service." example:"1b7f1f2e-3a4b-4c5d-8e9f-0a1b2c3d4e5f"`
	Name             string              `json:"name" doc:"Commercial name of the service." example:"Plan Fibra 600 Mbps"`
	Provider         string              `json:"provider" doc:"Name of the cheapest offer's provider, or an empty string when the service has no offers." example:"Movistar"`
	Category         string              `json:"category" doc:"Name of the category the service belongs to." example:"Internet"`
	Subcategory      string              `json:"subcategory" doc:"Name of the subcategory, or an empty string." example:"Fibra óptica"`
	Description      string              `json:"description" doc:"Free-form description, or an empty string." example:"Fibra óptica de 600 Mbps con equipo incluido"`
	MonthlyPrice     *money.Money        `json:"monthlyPrice" nullable:"true" doc:"Price of the cheapest offer, or null when the service has no offers."`
	InstallationCost *money.Money        `json:"installationCost" nullable:"true" doc:"Installation cost of the cheapest offer, or null when absent."`
	ContractMonths   *int                `json:"contractMonths" doc:"Length of the cheapest offer's contract in months, or null when unspecified." example:"12"`
	Rating           float64             `json:"rating" minimum:"0" maximum:"5" doc:"Average rating, 0 when the service has no reviews." example:"4.3"`
	ReviewCount      int64               `json:"reviewCount" minimum:"0" doc:"Number of published reviews." example:"8"`
	Specs            map[string]string   `json:"specs" doc:"Specifications keyed by name."`
	Benefits         []string            `json:"benefits" doc:"Additional costs summary of the cheapest offer, or an empty array."`
	Coverage         string              `json:"coverage" doc:"Coverage summary of the cheapest offer, or an empty string." example:"Cobertura nacional"`
	Image            string              `json:"image" doc:"Absolute URL of the service image, or an empty string."`
	Offers           []ServiceOffer      `json:"offers" doc:"Every provider offer, ordered by ascending price."`
	PriceHistory     []ServicePricePoint `json:"priceHistory" doc:"Every recorded price of every offer, oldest first."`
}

// ServiceOffer is one provider offer of a service.
type ServiceOffer struct {
	ID                     int32        `json:"id" doc:"Internal identifier of the offer." example:"1"`
	ProviderID             int32        `json:"providerId" doc:"Internal identifier of the provider." example:"1"`
	ProviderName           string       `json:"providerName" doc:"Name of the provider." example:"Movistar"`
	Price                  money.Money  `json:"price" doc:"Price, serialized as a JSON number." example:"29990"`
	Currency               string       `json:"currency" minLength:"3" maxLength:"3" doc:"ISO 4217 currency code." example:"CLP"`
	BillingPeriod          string       `json:"billingPeriod" doc:"Billing period label, or an empty string." example:"mensual"`
	InstallationCost       *money.Money `json:"installationCost" nullable:"true" doc:"One-time installation cost, or null when absent."`
	ContractPeriod         string       `json:"contractPeriod" doc:"Contract length label, or an empty string." example:"12 meses"`
	Available              bool         `json:"available" doc:"Whether the offer can currently be contracted." example:"true"`
	CoverageSummary        string       `json:"coverageSummary" doc:"Coverage summary, or an empty string." example:"Cobertura nacional"`
	AdditionalCostsSummary string       `json:"additionalCostsSummary" doc:"Additional costs summary, or an empty string."`
	ServiceURL             string       `json:"serviceUrl" format:"uri" doc:"Absolute URL of the offer, or an empty string." example:"https://movistar.cl/oferta/1"`
	LastUpdated            string       `json:"lastUpdated" format:"date" doc:"Day the offer was last updated, in ISO 8601." example:"2026-08-16"`
}

// PricePoint is one historical price of a service offer.
type ServicePricePoint struct {
	Date          string      `json:"date" format:"date" doc:"Day the price was recorded, in ISO 8601." example:"2026-08-16"`
	Price         money.Money `json:"price" doc:"Recorded price, serialized as a JSON number." example:"29990"`
	IsPromotional bool        `json:"isPromotional" doc:"Whether the recorded price was promotional." example:"false"`
}
