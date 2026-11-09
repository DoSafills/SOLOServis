package features

import (
	"context"
)

type Service struct {
	repository *Repository
}

func NewService(repository *Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) ProductID(ctx context.Context, value string) (int32, error) {
	if value == "" {
		return 0, errInvalidID
	}
	return s.repository.ResolveProductID(ctx, value)
}

func (s *Service) ServiceID(ctx context.Context, value string) (int32, error) {
	if value == "" {
		return 0, errInvalidID
	}
	return s.repository.ResolveServiceID(ctx, value)
}

func (s *Service) UserID(ctx context.Context, value int32) (int32, error) {
	if value <= 0 {
		return 0, errInvalidID
	}
	return s.repository.ResolveUserID(ctx, value)
}
