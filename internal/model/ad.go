package model

import (
	"strings"
	"time"
)

const (
	GenderMale   = "M"
	GenderFemale = "F"

	PlatformAndroid = "android"
	PlatformIOS     = "ios"
	PlatformWeb     = "web"

	DefaultLimit  = 5
	DefaultOffset = 0
)

var validGenders = map[string]struct{}{
	GenderMale:   {},
	GenderFemale: {},
}

var validPlatforms = map[string]struct{}{
	PlatformAndroid: {},
	PlatformIOS:     {},
	PlatformWeb:     {},
}

type Conditions struct {
	AgeStart *int     `json:"ageStart,omitempty"`
	AgeEnd   *int     `json:"ageEnd,omitempty"`
	Gender   []string `json:"gender,omitempty"`
	Country  []string `json:"country,omitempty"`
	Platform []string `json:"platform,omitempty"`
}

type Ad struct {
	ID         int64      `json:"id"`
	Title      string     `json:"title"`
	StartAt    time.Time  `json:"startAt"`
	EndAt      time.Time  `json:"endAt"`
	Conditions Conditions `json:"conditions"`
	CreatedAt  time.Time  `json:"createdAt"`
}

type CreateAdRequest struct {
	Title      string      `json:"title"`
	StartAt    string      `json:"startAt"`
	EndAt      string      `json:"endAt"`
	Conditions *Conditions `json:"conditions,omitempty"`
}

type UserProfile struct {
	Age      *int
	Gender   *string
	Country  *string
	Platform *string
}

type ListAdsQuery struct {
	Offset   int
	Limit    int
	Profile  UserProfile
}

type AdListItem struct {
	Title string    `json:"title"`
	EndAt time.Time `json:"endAt"`
}

type ListAdsResponse struct {
	Items []AdListItem `json:"items"`
}

func IsActive(ad Ad, now time.Time) bool {
	return ad.StartAt.Before(now) && ad.EndAt.After(now)
}

func (c Conditions) Matches(profile UserProfile) bool {
	if profile.Age != nil {
		if c.AgeStart != nil && *profile.Age < *c.AgeStart {
			return false
		}
		if c.AgeEnd != nil && *profile.Age > *c.AgeEnd {
			return false
		}
	}

	if profile.Gender != nil && len(c.Gender) > 0 {
		if !containsString(c.Gender, *profile.Gender) {
			return false
		}
	}

	if profile.Country != nil && len(c.Country) > 0 {
		if !containsString(c.Country, *profile.Country) {
			return false
		}
	}

	if profile.Platform != nil && len(c.Platform) > 0 {
		if !containsString(c.Platform, *profile.Platform) {
			return false
		}
	}

	return true
}

func ValidateCreateRequest(req CreateAdRequest) (title string, startAt, endAt time.Time, conditions Conditions, err error) {
	title = strings.TrimSpace(req.Title)
	if title == "" {
		return "", time.Time{}, time.Time{}, Conditions{}, invalid("title must be a non-empty string")
	}

	startAt, err = parseISO8601(req.StartAt, "startAt")
	if err != nil {
		return "", time.Time{}, time.Time{}, Conditions{}, err
	}

	endAt, err = parseISO8601(req.EndAt, "endAt")
	if err != nil {
		return "", time.Time{}, time.Time{}, Conditions{}, err
	}

	if !endAt.After(startAt) {
		return "", time.Time{}, time.Time{}, Conditions{}, invalid("endAt must be after startAt")
	}

	if req.Conditions != nil {
		if err := validateConditions(*req.Conditions); err != nil {
			return "", time.Time{}, time.Time{}, Conditions{}, err
		}
		conditions = normalizeConditions(*req.Conditions)
	}

	return title, startAt, endAt, conditions, nil
}

func ValidateListQuery(offset, limit int, age *int, gender, country, platform *string) (ListAdsQuery, error) {
	if offset < 0 {
		return ListAdsQuery{}, invalid("offset must be a non-negative integer")
	}

	if limit < 1 || limit > 100 {
		return ListAdsQuery{}, invalid("limit must be between 1 and 100")
	}

	profile := UserProfile{}

	if age != nil {
		if *age < 1 || *age > 100 {
			return ListAdsQuery{}, invalid("age must be between 1 and 100")
		}
		profile.Age = age
	}

	if gender != nil {
		g := strings.ToUpper(strings.TrimSpace(*gender))
		if _, ok := validGenders[g]; !ok {
			return ListAdsQuery{}, invalid("gender must be M or F")
		}
		profile.Gender = &g
	}

	if country != nil {
		c := strings.ToUpper(strings.TrimSpace(*country))
		if !isValidCountry(c) {
			return ListAdsQuery{}, invalid("country must be a valid ISO 3166-1 alpha-2 code")
		}
		profile.Country = &c
	}

	if platform != nil {
		p := strings.ToLower(strings.TrimSpace(*platform))
		if _, ok := validPlatforms[p]; !ok {
			return ListAdsQuery{}, invalid("platform must be android, ios, or web")
		}
		profile.Platform = &p
	}

	return ListAdsQuery{
		Offset:  offset,
		Limit:   limit,
		Profile: profile,
	}, nil
}

func validateConditions(c Conditions) error {
	if c.AgeStart != nil {
		if *c.AgeStart < 1 || *c.AgeStart > 100 {
			return invalid("conditions.ageStart must be between 1 and 100")
		}
	}
	if c.AgeEnd != nil {
		if *c.AgeEnd < 1 || *c.AgeEnd > 100 {
			return invalid("conditions.ageEnd must be between 1 and 100")
		}
	}
	if c.AgeStart != nil && c.AgeEnd != nil && *c.AgeEnd < *c.AgeStart {
		return invalid("conditions.ageEnd must be greater than or equal to conditions.ageStart")
	}

	for _, g := range c.Gender {
		if _, ok := validGenders[g]; !ok {
			return invalid("conditions.gender values must be M or F")
		}
	}

	for _, country := range c.Country {
		if !isValidCountry(strings.ToUpper(country)) {
			return invalid("conditions.country values must be valid ISO 3166-1 alpha-2 codes")
		}
	}

	for _, p := range c.Platform {
		if _, ok := validPlatforms[strings.ToLower(p)]; !ok {
			return invalid("conditions.platform values must be android, ios, or web")
		}
	}

	return nil
}

func parseISO8601(value, field string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, invalidf("%s must be a valid ISO 8601 timestamp", field)
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t.UTC(), nil
		}
	}

	return time.Time{}, invalidf("%s must be a valid ISO 8601 timestamp", field)
}

func containsString(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}

func normalizeConditions(c Conditions) Conditions {
	if len(c.Country) > 0 {
		countries := make([]string, len(c.Country))
		for i, country := range c.Country {
			countries[i] = strings.ToUpper(strings.TrimSpace(country))
		}
		c.Country = countries
	}

	if len(c.Platform) > 0 {
		platforms := make([]string, len(c.Platform))
		for i, platform := range c.Platform {
			platforms[i] = strings.ToLower(strings.TrimSpace(platform))
		}
		c.Platform = platforms
	}

	return c
}
