package service

import (
	"context"
	"sort"
	"time"

	"github.com/1chooo/ad-service/internal/model"
)

type AdStore interface {
	Create(ctx context.Context, ad *model.Ad) error
	ListActive(ctx context.Context, now time.Time) ([]model.Ad, error)
	RefreshCache(ctx context.Context, now time.Time) error
	ActiveAds() []model.Ad
	UpsertCache(ad model.Ad)
}

type AdService struct {
	store AdStore
	now   func() time.Time
}

func NewAdService(store AdStore) *AdService {
	return &AdService{
		store: store,
		now:   time.Now,
	}
}

func (s *AdService) WithClock(now func() time.Time) *AdService {
	s.now = now
	return s
}

func (s *AdService) CreateAd(ctx context.Context, req model.CreateAdRequest) (*model.Ad, error) {
	title, startAt, endAt, conditions, err := model.ValidateCreateRequest(req)
	if err != nil {
		return nil, err
	}

	ad := &model.Ad{
		Title:      title,
		StartAt:    startAt,
		EndAt:      endAt,
		Conditions: conditions,
	}

	if err := s.store.Create(ctx, ad); err != nil {
		return nil, err
	}

	now := s.now().UTC()
	if model.IsActive(*ad, now) {
		s.store.UpsertCache(*ad)
	}

	return ad, nil
}

func (s *AdService) ListAds(ctx context.Context, query model.ListAdsQuery) (*model.ListAdsResponse, error) {
	now := s.now().UTC()
	ads := s.store.ActiveAds()

	matched := make([]model.Ad, 0, len(ads))
	for _, ad := range ads {
		if !model.IsActive(ad, now) {
			continue
		}
		if !ad.Conditions.Matches(query.Profile) {
			continue
		}
		matched = append(matched, ad)
	}

	sort.Slice(matched, func(i, j int) bool {
		return matched[i].EndAt.Before(matched[j].EndAt)
	})

	if query.Offset >= len(matched) {
		return &model.ListAdsResponse{Items: []model.AdListItem{}}, nil
	}

	end := query.Offset + query.Limit
	if end > len(matched) {
		end = len(matched)
	}

	page := matched[query.Offset:end]
	items := make([]model.AdListItem, len(page))
	for i, ad := range page {
		items[i] = model.AdListItem{
			Title: ad.Title,
			EndAt: ad.EndAt,
		}
	}

	return &model.ListAdsResponse{Items: items}, nil
}

func (s *AdService) RefreshCache(ctx context.Context) error {
	return s.store.RefreshCache(ctx, s.now().UTC())
}
