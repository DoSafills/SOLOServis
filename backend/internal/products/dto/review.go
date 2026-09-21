package dto

import (
	"time"

	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
)

// ProductReview expone solo lo necesario para mostrar la reseña.
// No incluye email ni IDs internos del autor.
// AuthorVerified indica que el autor confirmó su email; no prueba una compra.
type ProductReview struct {
	Author         string `json:"author"`
	AuthorVerified bool   `json:"authorVerified"`
	Rating         int32  `json:"rating"`
	Title          string `json:"title"`
	Content        string `json:"content"`
	CreatedAt      string `json:"createdAt"`
}

func FromProductReview(review generated.ListProductReviewsRow) ProductReview {
	var title string
	if review.Title.Valid {
		title = review.Title.String
	}

	var content string
	if review.Content.Valid {
		content = review.Content.String
	}

	var createdAt string
	if review.CreatedAt.Valid {
		createdAt = review.CreatedAt.Time.Format(time.RFC3339)
	}

	return ProductReview{
		Author:         review.AuthorName,
		AuthorVerified: review.AuthorVerified,
		Rating:         review.Rating,
		Title:          title,
		Content:        content,
		CreatedAt:      createdAt,
	}
}
