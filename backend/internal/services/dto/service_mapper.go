package dto

import (
	"strconv"
	"strings"

	"github.com/DoSafills/SOLOServis/backend/internal/database/dbutil"
	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
)

// FromService arma el servicio a partir de su fila y de sus filas
// relacionadas. Precio, proveedor y condiciones salen de la oferta más
// barata (las ofertas llegan ordenadas por precio).
func FromService(
	service generated.ListServicesRow,
	offers []generated.ListServiceOffersRow,
	specifications []generated.ListServiceSpecificationsRow,
	history []generated.ListServicePriceHistoryRow,
) ServiceDetail {
	result := ServiceDetail{
		ID:           service.PublicID.String(),
		Name:         service.Name,
		Category:     service.CategoryName,
		Subcategory:  dbutil.Text(service.SubcategoryName),
		Description:  dbutil.Text(service.Description),
		Image:        dbutil.Text(service.ImageUrl),
		Rating:       dbutil.Float(service.Rating),
		ReviewCount:  service.ReviewCount,
		Specs:        make(map[string]string, len(specifications)),
		Benefits:     []string{},
		PriceHistory: make([]PricePoint, 0, len(history)),
	}

	for _, specification := range specifications {
		result.Specs[specification.Name] = dbutil.WithUnit(specification.Value, specification.Unit)
	}

	if len(offers) > 0 {
		offer := offers[0]

		result.Provider = offer.ProviderName
		result.MonthlyPrice = dbutil.Float(offer.Price)
		result.InstallationCost = dbutil.FloatPtr(offer.InstallationCost)
		result.Coverage = dbutil.Text(offer.CoverageSummary)

		if offer.ContractPeriod.Valid {
			result.ContractMonths = parseContractMonths(offer.ContractPeriod.String)
		}
	}

	for _, point := range history {
		price := dbutil.FloatPtr(point.Price)
		if price == nil {
			continue
		}

		result.PriceHistory = append(result.PriceHistory, PricePoint{
			Date:  dbutil.Date(point.RecordedAt),
			Price: *price,
		})
	}

	return result
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
