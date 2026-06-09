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
				Status:  model.StatusActive,
				Conditions: model.Conditions{
					Country: []string{"TW"},
				},
			},
			{
				ID:      2,
				Title:   "AD 31",
				StartAt: now.Add(-24 * time.Hour),
				EndAt:   time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC),
				Status:  model.StatusActive,
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
				Status:  model.StatusActive,
			},
			{
				ID:      4,
				Title:   "Expired",
				StartAt: now.Add(-72 * time.Hour),
				EndAt:   now.Add(-1 * time.Hour),
				Status:  model.StatusActive,
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

func TestListAdsSortingByBid(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	store := &mockStore{
		activeAds: []model.Ad{
			{
				ID:      1,
				Title:   "Low Bid",
				Bid:     1.0,
				StartAt: now.Add(-24 * time.Hour),
				EndAt:   time.Date(2026, 6, 30, 16, 0, 0, 0, time.UTC),
				Status:  model.StatusActive,
			},
			{
				ID:      2,
				Title:   "High Bid",
				Bid:     10.0,
				StartAt: now.Add(-24 * time.Hour),
				EndAt:   time.Date(2026, 6, 22, 1, 0, 0, 0, time.UTC),
				Status:  model.StatusActive,
			},
			{
				ID:      3,
				Title:   "Medium Bid",
				Bid:     5.0,
				StartAt: now.Add(-24 * time.Hour),
				EndAt:   time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC),
				Status:  model.StatusActive,
			},
		},
	}

	svc := NewAdService(store).WithClock(func() time.Time { return now })

	query, err := model.ValidateListQuery(0, 10, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("validate query: %v", err)
	}

	resp, err := svc.ListAds(context.Background(), query)
	if err != nil {
		t.Fatalf("list ads: %v", err)
	}

	if len(resp.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(resp.Items))
	}

	if resp.Items[0].Title != "High Bid" || resp.Items[1].Title != "Medium Bid" || resp.Items[2].Title != "Low Bid" {
		t.Fatalf("unexpected bid sort order: %+v", resp.Items)
	}
}

func TestListAdsExclusionTargeting(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	store := &mockStore{
		activeAds: []model.Ad{
			{
				ID:      1,
				Title:   "Exclude US",
				StartAt: now.Add(-24 * time.Hour),
				EndAt:   time.Date(2026, 6, 30, 16, 0, 0, 0, time.UTC),
				Status:  model.StatusActive,
				Conditions: model.Conditions{
					ExcludeCountry: []string{"US"},
				},
			},
			{
				ID:      2,
				Title:   "All Countries",
				StartAt: now.Add(-24 * time.Hour),
				EndAt:   time.Date(2026, 6, 30, 16, 0, 0, 0, time.UTC),
				Status:  model.StatusActive,
			},
		},
	}

	svc := NewAdService(store).WithClock(func() time.Time { return now })

	query, err := model.ValidateListQuery(0, 10, nil, nil, strPtr("US"), nil)
	if err != nil {
		t.Fatalf("validate query: %v", err)
	}

	resp, err := svc.ListAds(context.Background(), query)
	if err != nil {
		t.Fatalf("list ads: %v", err)
	}

	if len(resp.Items) != 1 || resp.Items[0].Title != "All Countries" {
		t.Fatalf("expected only all-countries ad for US user, got %+v", resp.Items)
	}
}

func TestListAdsPausedExcluded(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	store := &mockStore{
		activeAds: []model.Ad{
			{
				ID:      1,
				Title:   "Active Ad",
				StartAt: now.Add(-24 * time.Hour),
				EndAt:   time.Date(2026, 6, 30, 16, 0, 0, 0, time.UTC),
				Status:  model.StatusActive,
			},
			{
				ID:      2,
				Title:   "Paused Ad",
				StartAt: now.Add(-24 * time.Hour),
				EndAt:   time.Date(2026, 6, 30, 16, 0, 0, 0, time.UTC),
				Status:  model.StatusPaused,
			},
		},
	}

	svc := NewAdService(store).WithClock(func() time.Time { return now })

	query, err := model.ValidateListQuery(0, 10, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("validate query: %v", err)
	}

	resp, err := svc.ListAds(context.Background(), query)
	if err != nil {
		t.Fatalf("list ads: %v", err)
	}

	if len(resp.Items) != 1 || resp.Items[0].Title != "Active Ad" {
		t.Fatalf("expected only active ad, got %+v", resp.Items)
	}
}

func TestListAdsDailyBudget(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	budget := int64(2)
	store := &mockStore{
		activeAds: []model.Ad{
			{
				ID:          1,
				Title:       "Budget Ad",
				StartAt:     now.Add(-24 * time.Hour),
				EndAt:       time.Date(2026, 6, 30, 16, 0, 0, 0, time.UTC),
				Status:      model.StatusActive,
				DailyBudget: &budget,
			},
		},
	}

	svc := NewAdService(store).WithClock(func() time.Time { return now })

	query, err := model.ValidateListQuery(0, 10, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("validate query: %v", err)
	}

	resp, err := svc.ListAds(context.Background(), query)
	if err != nil {
		t.Fatalf("list ads: %v", err)
	}

	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item with budget, got %d", len(resp.Items))
	}

	svc.trackImpression(1, now)
	svc.trackImpression(1, now)

	resp, err = svc.ListAds(context.Background(), query)
	if err != nil {
		t.Fatalf("list ads: %v", err)
	}

	if len(resp.Items) != 0 {
		t.Fatalf("expected 0 items since budget exhausted, got %d", len(resp.Items))
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

func TestCreateAdWithAllFields(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	store := &mockStore{}
	svc := NewAdService(store).WithClock(func() time.Time { return now })

	status := model.StatusPaused
	bid := 3.5
	budget := int64(1000)

	ad, err := svc.CreateAd(context.Background(), model.CreateAdRequest{
		Title:          "Full Ad",
		Description:    "Full description",
		ImageUrl:       "https://example.com/img.jpg",
		LandingPageUrl: "https://example.com",
		Bid:            &bid,
		DailyBudget:    &budget,
		Status:         &status,
		StartAt:        "2026-06-10T03:00:00.000Z",
		EndAt:          "2026-06-30T16:00:00.000Z",
	})
	if err != nil {
		t.Fatalf("create ad: %v", err)
	}

	if ad.Description != "Full description" {
		t.Fatalf("expected description, got %q", ad.Description)
	}
	if ad.ImageUrl != "https://example.com/img.jpg" {
		t.Fatalf("expected imageUrl, got %q", ad.ImageUrl)
	}
	if ad.LandingPageUrl != "https://example.com" {
		t.Fatalf("expected landingPageUrl, got %q", ad.LandingPageUrl)
	}
	if ad.Bid != 3.5 {
		t.Fatalf("expected bid 3.5, got %f", ad.Bid)
	}
	if ad.DailyBudget == nil || *ad.DailyBudget != 1000 {
		t.Fatalf("expected dailyBudget 1000, got %v", ad.DailyBudget)
	}
	if ad.Status != model.StatusPaused {
		t.Fatalf("expected status paused, got %q", ad.Status)
	}

	if len(store.activeAds) != 0 {
		t.Fatalf("expected paused ad not to be added to cache")
	}
}

func TestBulkCreateAds(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	store := &mockStore{}
	svc := NewAdService(store).WithClock(func() time.Time { return now })

	resp, err := svc.BulkCreateAds(context.Background(), model.BulkCreateAdRequest{
		Ads: []model.CreateAdRequest{
			{
				Title:   "Ad 1",
				StartAt: "2026-06-10T03:00:00.000Z",
				EndAt:   "2026-06-30T16:00:00.000Z",
			},
			{
				Title:   "Ad 2",
				StartAt: "2026-06-10T03:00:00.000Z",
				EndAt:   "2026-06-30T16:00:00.000Z",
			},
			{
				Title:   "",
				StartAt: "2026-06-10T03:00:00.000Z",
				EndAt:   "2026-06-30T16:00:00.000Z",
			},
		},
	})
	if err != nil {
		t.Fatalf("bulk create: %v", err)
	}

	if len(resp.Ads) != 2 {
		t.Fatalf("expected 2 successful ads, got %d", len(resp.Ads))
	}

	if len(resp.Failures) != 1 || resp.Failures[0].Index != 2 {
		t.Fatalf("expected 1 failure at index 2, got %+v", resp.Failures)
	}
}

func strPtr(v string) *string { return &v }
