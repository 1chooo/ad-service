package model

import (
	"testing"
	"time"
)

func TestValidateCreateRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		req     CreateAdRequest
		wantErr string
	}{
		{
			name: "valid request with conditions",
			req: CreateAdRequest{
				Title:          "AD 55",
				Description:    "Great ad",
				ImageUrl:       "https://example.com/img.jpg",
				LandingPageUrl: "https://example.com",
				Bid:            float64Ptr(2.5),
				StartAt:        "2026-06-10T03:00:00.000Z",
				EndAt:          "2026-06-30T16:00:00.000Z",
				Conditions: &Conditions{
					AgeStart: intPtr(20),
					AgeEnd:   intPtr(30),
					Country:  []string{"TW", "JP"},
					Platform: []string{"android", "ios"},
				},
			},
		},
		{
			name: "empty title",
			req: CreateAdRequest{
				Title:   "  ",
				StartAt: "2026-06-10T03:00:00.000Z",
				EndAt:   "2026-06-30T16:00:00.000Z",
			},
			wantErr: "title must be a non-empty string",
		},
		{
			name: "endAt before startAt",
			req: CreateAdRequest{
				Title:   "AD",
				StartAt: "2026-06-30T16:00:00.000Z",
				EndAt:   "2026-06-10T03:00:00.000Z",
			},
			wantErr: "endAt must be after startAt",
		},
		{
			name: "invalid age range",
			req: CreateAdRequest{
				Title:   "AD",
				StartAt: "2026-06-10T03:00:00.000Z",
				EndAt:   "2026-06-30T16:00:00.000Z",
				Conditions: &Conditions{
					AgeStart: intPtr(30),
					AgeEnd:   intPtr(20),
				},
			},
			wantErr: "conditions.ageEnd must be greater than or equal to conditions.ageStart",
		},
		{
			name: "invalid country",
			req: CreateAdRequest{
				Title:   "AD",
				StartAt: "2026-06-10T03:00:00.000Z",
				EndAt:   "2026-06-30T16:00:00.000Z",
				Conditions: &Conditions{
					Country: []string{"XX"},
				},
			},
			wantErr: "conditions.country values must be valid ISO 3166-1 alpha-2 codes",
		},
		{
			name: "invalid exclude country",
			req: CreateAdRequest{
				Title:   "AD",
				StartAt: "2026-06-10T03:00:00.000Z",
				EndAt:   "2026-06-30T16:00:00.000Z",
				Conditions: &Conditions{
					ExcludeCountry: []string{"XX"},
				},
			},
			wantErr: "conditions.excludeCountry values must be valid ISO 3166-1 alpha-2 codes",
		},
		{
			name: "negative bid",
			req: CreateAdRequest{
				Title:   "AD",
				Bid:     float64Ptr(-1),
				StartAt: "2026-06-10T03:00:00.000Z",
				EndAt:   "2026-06-30T16:00:00.000Z",
			},
			wantErr: "bid must be a non-negative number",
		},
		{
			name: "negative daily budget",
			req: CreateAdRequest{
				Title:       "AD",
				DailyBudget: int64Ptr(-100),
				StartAt:     "2026-06-10T03:00:00.000Z",
				EndAt:       "2026-06-30T16:00:00.000Z",
			},
			wantErr: "dailyBudget must be a non-negative integer",
		},
		{
			name: "daypart only start set",
			req: CreateAdRequest{
				Title:   "AD",
				StartAt: "2026-06-10T03:00:00.000Z",
				EndAt:   "2026-06-30T16:00:00.000Z",
				Conditions: &Conditions{
					DaypartStart: strPtr("09:00"),
				},
			},
			wantErr: "conditions.daypartStart and daypartEnd must be both set or both unset",
		},
		{
			name: "invalid status",
			req: CreateAdRequest{
				Title:   "AD",
				Status:  strPtr("deleted"),
				StartAt: "2026-06-10T03:00:00.000Z",
				EndAt:   "2026-06-30T16:00:00.000Z",
			},
			wantErr: "status must be active, paused, or archived",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, _, _, _, _, _, _, _, _, _, err := ValidateCreateRequest(tt.req)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("expected error %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidateListQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		offset  int
		limit   int
		wantErr string
	}{
		{name: "valid defaults", offset: 0, limit: 5},
		{name: "negative offset", offset: -1, limit: 5, wantErr: "offset must be a non-negative integer"},
		{name: "limit too low", offset: 0, limit: 0, wantErr: "limit must be between 1 and 100"},
		{name: "limit too high", offset: 0, limit: 101, wantErr: "limit must be between 1 and 100"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := ValidateListQuery(tt.offset, tt.limit, nil, nil, nil, nil)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("expected error %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestConditionsMatches(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)

	t.Run("basic matching", func(t *testing.T) {
		t.Parallel()
		conditions := Conditions{
			AgeStart: intPtr(20),
			AgeEnd:   intPtr(30),
			Gender:   []string{"F"},
			Country:  []string{"TW", "JP"},
			Platform: []string{"ios"},
		}

		profile := UserProfile{
			Age:      intPtr(24),
			Gender:   strPtr("F"),
			Country:  strPtr("TW"),
			Platform: strPtr("ios"),
		}

		if !conditions.Matches(profile, now) {
			t.Fatal("expected profile to match conditions")
		}

		profile.Age = intPtr(31)
		if conditions.Matches(profile, now) {
			t.Fatal("expected age out of range to not match")
		}

		profile.Age = intPtr(24)
		profile.Country = strPtr("US")
		if conditions.Matches(profile, now) {
			t.Fatal("expected country mismatch to not match")
		}
	})

	t.Run("exclusion targeting", func(t *testing.T) {
		t.Parallel()
		conditions := Conditions{
			ExcludeCountry: []string{"US"},
			Gender:         []string{"F"},
		}

		profile := UserProfile{
			Country: strPtr("US"),
			Gender:  strPtr("F"),
		}

		if conditions.Matches(profile, now) {
			t.Fatal("expected excluded country to not match")
		}

		profile.Country = strPtr("TW")
		if !conditions.Matches(profile, now) {
			t.Fatal("expected non-excluded country to match")
		}
	})

	t.Run("dayparting within window", func(t *testing.T) {
		t.Parallel()
		daypartNow := time.Date(2026, 6, 15, 14, 30, 0, 0, time.UTC)
		conditions := Conditions{
			DaypartStart: strPtr("09:00"),
			DaypartEnd:   strPtr("17:00"),
		}

		profile := UserProfile{}

		if !conditions.Matches(profile, daypartNow) {
			t.Fatal("expected 14:30 to be within 09:00-17:00")
		}

		outsideNow := time.Date(2026, 6, 15, 20, 0, 0, 0, time.UTC)
		if conditions.Matches(profile, outsideNow) {
			t.Fatal("expected 20:00 to be outside 09:00-17:00")
		}
	})
}

func TestIsActive(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	ad := Ad{
		StartAt: time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
		EndAt:   time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
	}

	if !IsActive(ad, now) {
		t.Fatal("expected ad to be active")
	}

	if IsActive(ad, ad.StartAt) {
		t.Fatal("expected ad to be inactive at startAt boundary")
	}

	if IsActive(ad, ad.EndAt) {
		t.Fatal("expected ad to be inactive at endAt boundary")
	}

	t.Run("paused ad is not active", func(t *testing.T) {
		t.Parallel()
		pausedAd := Ad{
			StartAt: time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
			EndAt:   time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
			Status:  StatusPaused,
		}
		if IsActive(pausedAd, now) {
			t.Fatal("expected paused ad to be inactive")
		}
	})

	t.Run("archived ad is not active", func(t *testing.T) {
		t.Parallel()
		archivedAd := Ad{
			StartAt: time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
			EndAt:   time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
			Status:  StatusArchived,
		}
		if IsActive(archivedAd, now) {
			t.Fatal("expected archived ad to be inactive")
		}
	})
}

func intPtr(v int) *int             { return &v }
func strPtr(v string) *string       { return &v }
func float64Ptr(v float64) *float64 { return &v }
func int64Ptr(v int64) *int64       { return &v }
