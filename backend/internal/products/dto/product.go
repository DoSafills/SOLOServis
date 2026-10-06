package dto

import (
	"github.com/DoSafills/SOLOServis/backend/internal/database/dbutil"
	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
	"github.com/jackc/pgx/v5/pgtype"
)

type CreatedProduct struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ImageURL string `json:"imageUrl"`
}

type ProductListItem struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Brand       string         `json:"brand"`
	Model       string         `json:"model"`
	CategoryID  int32          `json:"categoryId"`
	Category    string         `json:"category"`
	Subcategory string         `json:"subcategory"`
	Description string         `json:"description"`
	Rating      float64        `json:"rating"`
	ReviewCount int64          `json:"reviewCount"`
	Images      []ProductImage `json:"images"`
	Offers      []ProductOffer `json:"offers"`
}

// ProductDetail amplía el ítem del listado; al embeberlo, sus campos se
// serializan al mismo nivel que los propios.
type ProductDetail struct {
	ProductListItem
	Specs             map[string]string `json:"specs"`
	PriceHistory      []PricePoint      `json:"priceHistory"`
	OfferPriceHistory []PricePoint      `json:"offerPriceHistory"`
}

type PricePoint struct {
	Date  string `json:"date"`
	Price string `json:"price"`
}

type ProductImage struct {
	URL       string `json:"url"`
	AltText   string `json:"altText"`
	SortOrder int32  `json:"sortOrder"`
}

type ProductOffer struct {
	StoreID      int32  `json:"storeId"`
	StoreName    string `json:"storeName"`
	Price        string `json:"price"`
	ListPrice    string `json:"listPrice"`
	Currency     string `json:"currency"`
	ShippingCost string `json:"shippingCost"`
	ShippingFree bool   `json:"shippingFree"`
	Available    bool   `json:"available"`
	Stock        *int32 `json:"stock"`
	Condition    string `json:"condition"`
	ProductURL   string `json:"productUrl"`
}

// splitCategory devuelve (categoría, subcategoría). Si la categoría del producto
// tiene padre, el padre es la categoría y la del producto es la subcategoría.
func splitCategory(categoryName string, parentName pgtype.Text) (string, string) {
	if parentName.Valid && parentName.String != "" {
		return parentName.String, categoryName
	}
	return categoryName, ""
}

func FromListProduct(
	row generated.ListProductsRow,
	images []generated.ProductImage,
	offers []generated.ListProductOffersRow,
) ProductListItem {
	category, subcategory := splitCategory(row.CategoryName, row.ParentCategoryName)

	item := ProductListItem{
		ID:          row.PublicID.String(),
		Name:        row.Name,
		Brand:       dbutil.Text(row.BrandName),
		Model:       dbutil.Text(row.Model),
		CategoryID:  row.CategoryID,
		Category:    category,
		Subcategory: subcategory,
		Description: dbutil.Text(row.Description),
		Rating:      dbutil.Float(row.Rating),
		ReviewCount: row.ReviewCount,
		Images:      make([]ProductImage, 0, len(images)),
		Offers:      make([]ProductOffer, 0, len(offers)),
	}

	for _, image := range images {
		item.Images = append(item.Images, ProductImage{
			URL:       image.ImageUrl,
			AltText:   dbutil.Text(image.AltText),
			SortOrder: image.SortOrder,
		})
	}

	for _, offer := range offers {
		item.Offers = append(item.Offers, ProductOffer{
			StoreID:      offer.StoreID,
			StoreName:    offer.StoreName,
			Price:        dbutil.NumericString(offer.Price),
			ListPrice:    dbutil.NumericString(offer.ListPrice),
			Currency:     offer.Currency,
			ShippingCost: dbutil.NumericString(offer.ShippingCost),
			ShippingFree: offer.ShippingFree,
			Available:    offer.Available,
			Stock:        dbutil.Int4Ptr(offer.Stock),
			Condition:    offer.Condition,
			ProductURL:   dbutil.Text(offer.ProductUrl),
		})
	}

	return item
}

func NewProductDetail(
	item ProductListItem,
	specifications []generated.ListProductSpecificationsRow,
	history []generated.ListProductPriceHistoryRow,
) ProductDetail {
	detail := ProductDetail{
		ProductListItem:   item,
		Specs:             make(map[string]string, len(specifications)),
		PriceHistory:      make([]PricePoint, 0, len(history)),
		OfferPriceHistory: []PricePoint{},
	}

	for _, specification := range specifications {
		detail.Specs[specification.Name] = dbutil.WithUnit(specification.Value, specification.Unit)
	}

	for _, point := range history {
		pricePoint := PricePoint{
			Date:  dbutil.Date(point.RecordedAt),
			Price: dbutil.NumericString(point.Price),
		}

		detail.PriceHistory = append(detail.PriceHistory, pricePoint)

		if point.IsPromotional {
			detail.OfferPriceHistory = append(detail.OfferPriceHistory, pricePoint)
		}
	}

	return detail
}
