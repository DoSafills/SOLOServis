package dto

import (
	"strconv"
	"strings"

	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
	"github.com/DoSafills/SOLOServis/backend/internal/money"
)

// Relations holds the child rows of a single service. The three slices come from
// batched queries, so the same shape serves both the listing and the detail
// endpoint.
type Relations struct {
	Offers         []generated.ListServiceOffersByServiceIDsRow
	Specifications map[string]string
	PriceHistory   []generated.ListServicePriceHistoryByServiceIDsRow
}

// NewRelations groups the batched rows of several services by service
// identifier, which is what lets the listing endpoint run three queries in
// total instead of three per service.
func NewRelations(
	offers []generated.ListServiceOffersByServiceIDsRow,
	specifications []generated.ListServiceSpecificationsByServiceIDsRow,
	history []generated.ListServicePriceHistoryByServiceIDsRow,
) map[int32]Relations {
	grouped := make(map[int32]Relations)

	ensure := func(serviceID int32) Relations {
		existing, ok := grouped[serviceID]
		if !ok {
			existing = Relations{
				Offers:         nil,
				Specifications: map[string]string{},
				PriceHistory:   nil,
			}
			grouped[serviceID] = existing
		}

		return existing
	}

	for _, offer := range offers {
		current := ensure(offer.ServiceID)
		current.Offers = append(current.Offers, offer)
		grouped[offer.ServiceID] = current
	}

	for _, specification := range specifications {
		current := ensure(specification.ServiceID)
		current.Specifications[specification.Name] = specification.Value
		grouped[specification.ServiceID] = current
	}

	for _, point := range history {
		current := ensure(point.ServiceID)
		current.PriceHistory = append(current.PriceHistory, point)
		grouped[point.ServiceID] = current
	}

	return grouped
}

// ListServices maps listing rows into the API representation. The result is
// never nil.
func ListServices(rows []generated.ListServicesRow, relations map[int32]Relations) []ServiceDetail {
	services := make([]ServiceDetail, 0, len(rows))

	for _, row := range rows {
		services = append(services, FromService(
			asDetailRow(row),
			relations[row.ID],
		))
	}

	return services
}

// FromService maps a service row plus its relations into the API
// representation. The zero Relations yields a service with no offers, no
// specifications and no price history rather than nil slices.
func FromService(service generated.GetServiceByPublicIDRow, relations Relations) ServiceDetail {
	result := ServiceDetail{
		ID:           service.PublicID.String(),
		Name:         service.Name,
		Category:     service.CategoryName,
		Specs:        map[string]string{},
		Benefits:     []string{},
		Offers:       []ServiceOffer{},
		PriceHistory: []ServicePricePoint{},
		ReviewCount:  service.ReviewCount,
	}

	if service.Description.Valid {
		result.Description = service.Description.String
	}

	if service.ImageUrl.Valid {
		result.Image = service.ImageUrl.String
	}

	if service.SubcategoryName.Valid {
		result.Subcategory = service.SubcategoryName.String
	}

	if service.Rating.Valid {
		if value, err := service.Rating.Float64Value(); err == nil && value.Valid {
			result.Rating = value.Float64
		}
	}

	for name, value := range relations.Specifications {
		result.Specs[name] = value
	}

	result.Offers = make([]ServiceOffer, 0, len(relations.Offers))
	for _, offer := range relations.Offers {
		result.Offers = append(result.Offers, fromOffer(offer))
	}

	// The batched queries order offers by ascending price, so the first one is
	// the cheapest and is the one surfaced as the service headline price.
	if len(result.Offers) > 0 {
		cheapest := relations.Offers[0]

		result.Provider = cheapest.ProviderName

		if price := money.FromNumeric(cheapest.Price); price.Valid() {
			result.MonthlyPrice = &price
		}

		if cost := money.FromNumeric(cheapest.InstallationCost); cost.Valid() {
			result.InstallationCost = &cost
		}

		if cheapest.ContractPeriod.Valid {
			result.ContractMonths = ParseContractMonths(cheapest.ContractPeriod.String)
		}

		if cheapest.CoverageSummary.Valid {
			result.Coverage = cheapest.CoverageSummary.String
		}

		// Benefits is intentionally left empty: the previous implementation
		// never populated it, and deriving it from
		// additional_costs_summary would change the response contract.
	}

	result.PriceHistory = make([]ServicePricePoint, 0, len(relations.PriceHistory))
	for _, point := range relations.PriceHistory {
		price := money.FromNumeric(point.Price)
		if !price.Valid() {
			continue
		}

		var date string
		if point.RecordedAt.Valid {
			date = point.RecordedAt.Time.Format("2006-01-02")
		}

		result.PriceHistory = append(result.PriceHistory, ServicePricePoint{
			Date:          date,
			Price:         price,
			IsPromotional: point.IsPromotional,
		})
	}

	return result
}

func fromOffer(offer generated.ListServiceOffersByServiceIDsRow) ServiceOffer {
	var installationCost *money.Money
	if value := money.FromNumeric(offer.InstallationCost); value.Valid() {
		installationCost = &value
	}

	var billingPeriod string
	if offer.BillingPeriod.Valid {
		billingPeriod = offer.BillingPeriod.String
	}

	var contractPeriod string
	if offer.ContractPeriod.Valid {
		contractPeriod = offer.ContractPeriod.String
	}

	var coverage string
	if offer.CoverageSummary.Valid {
		coverage = offer.CoverageSummary.String
	}

	var additionalCosts string
	if offer.AdditionalCostsSummary.Valid {
		additionalCosts = offer.AdditionalCostsSummary.String
	}

	var serviceURL string
	if offer.ServiceUrl.Valid {
		serviceURL = offer.ServiceUrl.String
	}

	var lastUpdated string
	if offer.LastUpdated.Valid {
		lastUpdated = offer.LastUpdated.Time.Format("2006-01-02")
	}

	return ServiceOffer{
		ID:                     offer.ID,
		ProviderID:             offer.ProviderID,
		ProviderName:           offer.ProviderName,
		Price:                  money.FromNumeric(offer.Price),
		Currency:               offer.Currency,
		BillingPeriod:          billingPeriod,
		InstallationCost:       installationCost,
		ContractPeriod:         contractPeriod,
		Available:              offer.Available,
		CoverageSummary:        coverage,
		AdditionalCostsSummary: additionalCosts,
		ServiceURL:             serviceURL,
		LastUpdated:            lastUpdated,
	}
}

// ParseContractMonths extracts the leading month count from a free-form
// contract label such as "12 meses". It returns nil when no positive count can
// be read.
func ParseContractMonths(value string) *int {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(value)))
	if len(fields) == 0 {
		return nil
	}

	months, err := strconv.Atoi(fields[0])
	if err != nil || months <= 0 {
		return nil
	}

	return &months
}

// asDetailRow converts a listing row into the detail row shape. The two rows
// generated by sqlc have identical columns; they are distinct types only
// because they come from different queries.
func asDetailRow(row generated.ListServicesRow) generated.GetServiceByPublicIDRow {
	return generated.GetServiceByPublicIDRow{
		ID:              row.ID,
		PublicID:        row.PublicID,
		CategoryID:      row.CategoryID,
		Name:            row.Name,
		Description:     row.Description,
		ImageUrl:        row.ImageUrl,
		Active:          row.Active,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		CategoryName:    row.CategoryName,
		SubcategoryName: row.SubcategoryName,
		Rating:          row.Rating,
		ReviewCount:     row.ReviewCount,
	}
}
