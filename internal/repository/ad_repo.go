package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/1chooo/ad-service/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

const migrationSQL = `
CREATE TABLE IF NOT EXISTS ads (
  id               BIGSERIAL PRIMARY KEY,
  title            TEXT NOT NULL,
  description      TEXT NOT NULL DEFAULT '',
  image_url        TEXT NOT NULL DEFAULT '',
  landing_page_url TEXT NOT NULL DEFAULT '',
  bid              DOUBLE PRECISION NOT NULL DEFAULT 0,
  daily_budget     BIGINT,
  status           TEXT NOT NULL DEFAULT 'active',
  start_at         TIMESTAMPTZ NOT NULL,
  end_at           TIMESTAMPTZ NOT NULL,
  conditions       JSONB NOT NULL DEFAULT '{}',
  created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

DO $$ BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_indexes WHERE indexname = 'idx_ads_active'
  ) THEN
    CREATE INDEX idx_ads_active ON ads (start_at, end_at);
  END IF;
END $$;

DO $$ BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_indexes WHERE indexname = 'idx_ads_status'
  ) THEN
    CREATE INDEX idx_ads_status ON ads (status);
  END IF;
END $$;
`

type AdRepository struct {
	pool  *pgxpool.Pool
	cache *ActiveAdCache
}

func NewAdRepository(ctx context.Context, databaseURL string) (*AdRepository, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	if _, err := pool.Exec(ctx, migrationSQL); err != nil {
		pool.Close()
		return nil, fmt.Errorf("run migration: %w", err)
	}

	return &AdRepository{
		pool:  pool,
		cache: NewActiveAdCache(),
	}, nil
}

func (r *AdRepository) Close() {
	r.pool.Close()
}

func (r *AdRepository) Create(ctx context.Context, ad *model.Ad) error {
	conditionsJSON, err := json.Marshal(ad.Conditions)
	if err != nil {
		return fmt.Errorf("marshal conditions: %w", err)
	}

	err = r.pool.QueryRow(ctx, `
		INSERT INTO ads (title, description, image_url, landing_page_url, bid, daily_budget, status, start_at, end_at, conditions)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at
	`, ad.Title, ad.Description, ad.ImageUrl, ad.LandingPageUrl, ad.Bid, ad.DailyBudget, ad.Status, ad.StartAt, ad.EndAt, conditionsJSON).Scan(&ad.ID, &ad.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert ad: %w", err)
	}

	return nil
}

func (r *AdRepository) ListActive(ctx context.Context, now time.Time) ([]model.Ad, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, title, description, image_url, landing_page_url, bid, daily_budget, status, start_at, end_at, conditions, created_at
		FROM ads
		WHERE start_at < $1 AND end_at > $1 AND status = 'active'
		ORDER BY end_at ASC
	`, now)
	if err != nil {
		return nil, fmt.Errorf("query active ads: %w", err)
	}
	defer rows.Close()

	var ads []model.Ad
	for rows.Next() {
		ad, err := scanAd(rows)
		if err != nil {
			return nil, err
		}
		ads = append(ads, ad)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active ads: %w", err)
	}

	return ads, nil
}

func (r *AdRepository) RefreshCache(ctx context.Context, now time.Time) error {
	ads, err := r.ListActive(ctx, now)
	if err != nil {
		return err
	}
	r.cache.Refresh(ads)
	return nil
}

func (r *AdRepository) ActiveAds() []model.Ad {
	return r.cache.Active()
}

func (r *AdRepository) UpsertCache(ad model.Ad) {
	r.cache.Upsert(ad)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAd(row rowScanner) (model.Ad, error) {
	var ad model.Ad
	var conditionsJSON []byte

	if err := row.Scan(&ad.ID, &ad.Title, &ad.Description, &ad.ImageUrl, &ad.LandingPageUrl, &ad.Bid, &ad.DailyBudget, &ad.Status, &ad.StartAt, &ad.EndAt, &conditionsJSON, &ad.CreatedAt); err != nil {
		return model.Ad{}, fmt.Errorf("scan ad: %w", err)
	}

	if len(conditionsJSON) > 0 {
		if err := json.Unmarshal(conditionsJSON, &ad.Conditions); err != nil {
			return model.Ad{}, fmt.Errorf("unmarshal conditions: %w", err)
		}
	}

	if ad.Status == "" {
		ad.Status = model.StatusActive
	}

	return ad, nil
}

type ActiveAdCache struct {
	mu   sync.RWMutex
	byID map[int64]model.Ad
}

func NewActiveAdCache() *ActiveAdCache {
	return &ActiveAdCache{
		byID: make(map[int64]model.Ad),
	}
}

func (c *ActiveAdCache) Active() []model.Ad {
	c.mu.RLock()
	defer c.mu.RUnlock()

	ads := make([]model.Ad, 0, len(c.byID))
	for _, ad := range c.byID {
		ads = append(ads, ad)
	}
	return ads
}

func (c *ActiveAdCache) Upsert(ad model.Ad) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.byID[ad.ID] = ad
}

func (c *ActiveAdCache) Refresh(ads []model.Ad) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.byID = make(map[int64]model.Ad, len(ads))
	for _, ad := range ads {
		c.byID[ad.ID] = ad
	}
}
