package dto

import (
	"strconv"
	"strings"

	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
)

func FromService(
	service generated.GetServiceByPublicIDRow,
	offers []generated.ListServiceOffersRow,
	specifications []generated.ListServiceSpecificationsRow,
	history []generated.ListServicePriceHistoryRow,
) ServiceDetail {
	result := ServiceDetail{
		ID:           service.PublicID.String(),
		Name:         service.Name,
		Category:     service.CategoryName,
		Specs:        make(map[string]string),
		Benefits:     []string{},
		PriceHistory: []PricePoint{},
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

	for _, specification := range specifications {
		result.Specs[specification.Name] = specification.Value
	}

	if len(offers) > 0 {
		offer := offers[0]

		result.Provider = offer.ProviderName

		if offer.Price.Valid {
			if value, err := offer.Price.Float64Value(); err == nil && value.Valid {
				result.MonthlyPrice = value.Float64
			}
		}
		result.Currency = offer.Currency
		if offer.BillingPeriod.Valid {
			result.BillingPeriod = offer.BillingPeriod.String
		}

		if offer.InstallationCost.Valid {
			if value, err := offer.InstallationCost.Float64Value(); err == nil && value.Valid {
				result.InstallationCost = &value.Float64
			}
		}

		if offer.ContractPeriod.Valid {
			result.ContractPeriod = offer.ContractPeriod.String
			result.ContractMonths = parseContractMonths(offer.ContractPeriod.String)
		}

		if offer.CoverageSummary.Valid {
			result.Coverage = offer.CoverageSummary.String
		}
	}

	for _, point := range history {
		if !point.Price.Valid {
			continue
		}

		value, err := point.Price.Float64Value()
		if err != nil || !value.Valid {
			continue
		}

		date := ""
		if point.RecordedAt.Valid {
			date = point.RecordedAt.Time.Format("2006-01-02")
		}

		result.PriceHistory = append(result.PriceHistory, PricePoint{
			Date:  date,
			Price: value.Float64,
		})
	}

	return result
}

func FromServiceList(
	service generated.ListServicesRow,
	offers []generated.ListServiceOffersRow,
	specifications []generated.ListServiceSpecificationsRow,
	history []generated.ListServicePriceHistoryRow,
) ServiceDetail {
	return FromService(
		generated.GetServiceByPublicIDRow{
			ID:              service.ID,
			PublicID:        service.PublicID,
			CategoryID:      service.CategoryID,
			Name:            service.Name,
			Description:     service.Description,
			ImageUrl:        service.ImageUrl,
			Active:          service.Active,
			CreatedAt:       service.CreatedAt,
			UpdatedAt:       service.UpdatedAt,
			CategoryName:    service.CategoryName,
			SubcategoryName: service.SubcategoryName,
			Rating:          service.Rating,
			ReviewCount:     service.ReviewCount,
		},
		offers,
		specifications,
		history,
	)
}

func parseContractMonths(value string) *int {
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
