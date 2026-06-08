package service

import (
	"context"
	"testing"
	"time"

	"github.com/1chooo/ad-service/internal/model"
)

type mockStore struct {
	ads       []model.Ad
	activeAds []model.Ad
}

func (m *mockStore) Create(_ context.Context, ad *model.Ad) error {
	ad.ID = int64(len(m.ads) + 1)
	ad.CreatedAt = time.Now().UTC()
	m.ads = append(m.ads, *ad)
	return nil
}

func (m *mockStore) ListActive(_ context.Context, now time.Time) ([]model.Ad, error) {
	var active []model.Ad
	for _, ad := range m.ads {
		if model.IsActive(ad, now) {
			active = append(active, ad)
		}
	}
	return active, nil
}

func (m *mockStore) RefreshCache(_ context.Context, now time.Time) error {
	active, err := m.ListActive(context.Background(), now)
	if err != nil {
		return err
	}
	m.activeAds = active
	return nil
}

func (m *mockStore) ActiveAds() []model.Ad {
	if len(m.activeAds) > 0 {
		return append([]model.Ad(nil), m.activeAds...)
	}
	return append([]model.Ad(nil), m.ads...)
}

func (m *mockStore) UpsertCache(ad model.Ad) {
	for i, existing := range m.activeAds {
		if existing.ID == ad.ID {
			m.activeAds[i] = ad
			return
		}
	}
	m.activeAds = append(m.activeAds, ad)
}

func TestListAdsFilteringSortingPagination(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	store := &mockStore{
		activeAds: []model.Ad{
			{
				ID:      1,
				Title:   "AD 1",
				StartAt: now.Add(-24 * time.Hour),
				EndAt:   time.Date(2026, 6, 22, 1, 0, 0, 0, time.UTC),
				Conditions: model.Conditions{
					Country: []string{"TW"},
				},
			},
			{
				ID:      2,
				Title:   "AD 31",
				StartAt: now.Add(-24 * time.Hour),
				EndAt:   time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC),
				Conditions: model.Conditions{
					Gender:  []string{"M"},
					Country: []string{"JP"},
				},
			},
			{
				ID:      3,
				Title:   "AD 10",
				StartAt: now.Add(-24 * time.Hour),
				EndAt:   time.Date(2026, 6, 30, 16, 0, 0, 0, time.UTC),
			},
			{
				ID:      4,
				Title:   "Expired",
				StartAt: now.Add(-72 * time.Hour),
				EndAt:   now.Add(-1 * time.Hour),
			},
		},
	}

	svc := NewAdService(store).WithClock(func() time.Time { return now })

	query, err := model.ValidateListQuery(0, 3, nil, strPtr("F"), strPtr("TW"), nil)
	if err != nil {
		t.Fatalf("validate query: %v", err)
	}

	resp, err := svc.ListAds(context.Background(), query)
	if err != nil {
		t.Fatalf("list ads: %v", err)
	}

	if len(resp.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(resp.Items))
	}

	if resp.Items[0].Title != "AD 1" || resp.Items[1].Title != "AD 10" {
		t.Fatalf("unexpected sort order: %+v", resp.Items)
	}

	// AD 31 targets M/JP only and should be excluded.
	for _, item := range resp.Items {
		if item.Title == "AD 31" {
			t.Fatal("expected AD 31 to be filtered out")
		}
	}

	query, err = model.ValidateListQuery(1, 1, nil, strPtr("F"), strPtr("TW"), nil)
	if err != nil {
		t.Fatalf("validate query: %v", err)
	}

	resp, err = svc.ListAds(context.Background(), query)
	if err != nil {
		t.Fatalf("list ads: %v", err)
	}

	if len(resp.Items) != 1 || resp.Items[0].Title != "AD 10" {
		t.Fatalf("unexpected pagination result: %+v", resp.Items)
	}
}

func TestCreateAdAddsToCacheWhenActive(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	store := &mockStore{}
	svc := NewAdService(store).WithClock(func() time.Time { return now })

	ad, err := svc.CreateAd(context.Background(), model.CreateAdRequest{
		Title:   "New AD",
		StartAt: "2026-06-10T03:00:00.000Z",
		EndAt:   "2026-06-30T16:00:00.000Z",
	})
	if err != nil {
		t.Fatalf("create ad: %v", err)
	}

	if len(store.activeAds) != 1 || store.activeAds[0].Title != ad.Title {
		t.Fatalf("expected active cache to contain created ad")
	}
}

func strPtr(v string) *string { return &v }
