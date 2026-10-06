package dto

import (
	"strconv"

	"github.com/DoSafills/SOLOServis/backend/internal/database/dbutil"
	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
)

type Store struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Logo         string  `json:"logo"`
	Rating       float64 `json:"rating"`
	ReviewCount  int64   `json:"reviewCount"`
	ProductCount int64   `json:"productCount"`
	Reputation   string  `json:"reputation"`
	DispatchTime string  `json:"dispatchTime"`
	Conditions   string  `json:"conditions"`
	Website      string  `json:"website"`
}

func FromStore(row generated.ListStoresRow) Store {
	return Store{
		ID:           strconv.Itoa(int(row.ID)),
		Name:         row.Name,
		Logo:         dbutil.Text(row.LogoUrl),
		Rating:       dbutil.Float(row.Rating),
		ProductCount: row.ProductCount,
		Reputation:   dbutil.Text(row.Reputation),
		DispatchTime: dbutil.Text(row.ShippingInformation),
		Conditions:   dbutil.Text(row.GeneralConditions),
		Website:      dbutil.Text(row.WebsiteUrl),
	}
}
