package features

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

type Service struct {
	repository *Repository
}

func NewService(repository *Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) ProductID(ctx context.Context, value string) (int32, error) {
	if value == "" {
		return 0, errors.New("productId is required")
	}
	return s.repository.ResolveProductID(ctx, value)
}

func (s *Service) ServiceID(ctx context.Context, value string) (int32, error) {
	if value == "" {
		return 0, errors.New("serviceId is required")
	}
	return s.repository.ResolveServiceID(ctx, value)
}

func (s *Service) UserID(ctx context.Context, value int32) (int32, error) {
	if value <= 0 {
		return 0, errors.New("userId is required")
	}
	return s.repository.ResolveUserID(ctx, value)
}

func isNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
