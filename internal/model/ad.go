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

	StatusActive   = "active"
	StatusPaused   = "paused"
	StatusArchived = "archived"

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

var validStatuses = map[string]struct{}{
	StatusActive:   {},
	StatusPaused:   {},
	StatusArchived: {},
}

type Conditions struct {
	AgeStart        *int     `json:"ageStart,omitempty"`
	AgeEnd          *int     `json:"ageEnd,omitempty"`
	Gender          []string `json:"gender,omitempty"`
	Country         []string `json:"country,omitempty"`
	Platform        []string `json:"platform,omitempty"`
	ExcludeGender   []string `json:"excludeGender,omitempty"`
	ExcludeCountry  []string `json:"excludeCountry,omitempty"`
	ExcludePlatform []string `json:"excludePlatform,omitempty"`
	DaypartStart    *string  `json:"daypartStart,omitempty"`
	DaypartEnd      *string  `json:"daypartEnd,omitempty"`
}

type Ad struct {
	ID             int64      `json:"id"`
	Title          string     `json:"title"`
	Description    string     `json:"description,omitempty"`
	ImageUrl       string     `json:"imageUrl,omitempty"`
	LandingPageUrl string     `json:"landingPageUrl,omitempty"`
	Bid            float64    `json:"bid,omitempty"`
	DailyBudget    *int64     `json:"dailyBudget,omitempty"`
	Status         string     `json:"status"`
	StartAt        time.Time  `json:"startAt"`
	EndAt          time.Time  `json:"endAt"`
	Conditions     Conditions `json:"conditions"`
	CreatedAt      time.Time  `json:"createdAt"`
}

type CreateAdRequest struct {
	Title          string      `json:"title"`
	Description    string      `json:"description,omitempty"`
	ImageUrl       string      `json:"imageUrl,omitempty"`
	LandingPageUrl string      `json:"landingPageUrl,omitempty"`
	Bid            *float64    `json:"bid,omitempty"`
	DailyBudget    *int64      `json:"dailyBudget,omitempty"`
	Status         *string     `json:"status,omitempty"`
	StartAt        string      `json:"startAt"`
	EndAt          string      `json:"endAt"`
	Conditions     *Conditions `json:"conditions,omitempty"`
}

type BulkCreateAdRequest struct {
	Ads []CreateAdRequest `json:"ads"`
}

type BulkCreateAdResponse struct {
	Ads      []Ad             `json:"ads"`
	Failures []BulkCreateFail `json:"failures,omitempty"`
}

type BulkCreateFail struct {
	Index int    `json:"index"`
	Error string `json:"error"`
}

type UserProfile struct {
	Age      *int
	Gender   *string
	Country  *string
	Platform *string
}

type ListAdsQuery struct {
	Offset  int
	Limit   int
	Profile UserProfile
}

type AdListItem struct {
	Title          string    `json:"title"`
	Description    string    `json:"description,omitempty"`
	ImageUrl       string    `json:"imageUrl,omitempty"`
	LandingPageUrl string    `json:"landingPageUrl,omitempty"`
	EndAt          time.Time `json:"endAt"`
}

type ListAdsResponse struct {
	Items []AdListItem `json:"items"`
}

type SpendRecord struct {
	Impressions int64
	Date        string
}

func IsActive(ad Ad, now time.Time) bool {
	if ad.Status == StatusPaused || ad.Status == StatusArchived {
		return false
	}
	return ad.StartAt.Before(now) && ad.EndAt.After(now)
}

func (c Conditions) Matches(profile UserProfile, now time.Time) bool {
	if profile.Age != nil {
		if c.AgeStart != nil && *profile.Age < *c.AgeStart {
			return false
		}
		if c.AgeEnd != nil && *profile.Age > *c.AgeEnd {
			return false
		}
	}

	if profile.Gender != nil {
		if containsString(c.ExcludeGender, *profile.Gender) {
			return false
		}
		if len(c.Gender) > 0 && !containsString(c.Gender, *profile.Gender) {
			return false
		}
	}

	if profile.Country != nil {
		if containsString(c.ExcludeCountry, *profile.Country) {
			return false
		}
		if len(c.Country) > 0 && !containsString(c.Country, *profile.Country) {
			return false
		}
	}

	if profile.Platform != nil {
		if containsString(c.ExcludePlatform, *profile.Platform) {
			return false
		}
		if len(c.Platform) > 0 && !containsString(c.Platform, *profile.Platform) {
			return false
		}
	}

	if c.DaypartStart != nil && c.DaypartEnd != nil {
		currentMinutes := now.Hour()*60 + now.Minute()
		startMinutes := parseDaypartMinutes(*c.DaypartStart)
		endMinutes := parseDaypartMinutes(*c.DaypartEnd)
		if startMinutes >= 0 && endMinutes >= 0 {
			if startMinutes <= endMinutes {
				if currentMinutes < startMinutes || currentMinutes > endMinutes {
					return false
				}
			} else {
				if currentMinutes < startMinutes && currentMinutes > endMinutes {
					return false
				}
			}
		}
	}

	return true
}

func ValidateCreateRequest(req CreateAdRequest) (title string, startAt, endAt time.Time, conditions Conditions, description, imageUrl, landingPageUrl string, bid float64, dailyBudget *int64, status string, err error) {
	title = strings.TrimSpace(req.Title)
	if title == "" {
		return "", time.Time{}, time.Time{}, Conditions{}, "", "", "", 0, nil, "", invalid("title must be a non-empty string")
	}

	startAt, err = parseISO8601(req.StartAt, "startAt")
	if err != nil {
		return "", time.Time{}, time.Time{}, Conditions{}, "", "", "", 0, nil, "", err
	}

	endAt, err = parseISO8601(req.EndAt, "endAt")
	if err != nil {
		return "", time.Time{}, time.Time{}, Conditions{}, "", "", "", 0, nil, "", err
	}

	if !endAt.After(startAt) {
		return "", time.Time{}, time.Time{}, Conditions{}, "", "", "", 0, nil, "", invalid("endAt must be after startAt")
	}

	if req.DailyBudget != nil && *req.DailyBudget < 0 {
		return "", time.Time{}, time.Time{}, Conditions{}, "", "", "", 0, nil, "", invalid("dailyBudget must be a non-negative integer")
	}

	if req.Bid != nil {
		if *req.Bid < 0 {
			return "", time.Time{}, time.Time{}, Conditions{}, "", "", "", 0, nil, "", invalid("bid must be a non-negative number")
		}
		bid = *req.Bid
	}

	st := StatusActive
	if req.Status != nil {
		s := strings.ToLower(strings.TrimSpace(*req.Status))
		if _, ok := validStatuses[s]; !ok {
			return "", time.Time{}, time.Time{}, Conditions{}, "", "", "", 0, nil, "", invalid("status must be active, paused, or archived")
		}
		st = s
	}
	status = st

	if req.Conditions != nil {
		if err := validateConditions(*req.Conditions); err != nil {
			return "", time.Time{}, time.Time{}, Conditions{}, "", "", "", 0, nil, "", err
		}
		conditions = normalizeConditions(*req.Conditions)
	}

	description = req.Description
	imageUrl = req.ImageUrl
	landingPageUrl = req.LandingPageUrl
	dailyBudget = req.DailyBudget

	return title, startAt, endAt, conditions, description, imageUrl, landingPageUrl, bid, dailyBudget, status, nil
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

	for _, g := range c.ExcludeGender {
		if _, ok := validGenders[g]; !ok {
			return invalid("conditions.excludeGender values must be M or F")
		}
	}

	for _, country := range c.Country {
		if !isValidCountry(strings.ToUpper(country)) {
			return invalid("conditions.country values must be valid ISO 3166-1 alpha-2 codes")
		}
	}

	for _, country := range c.ExcludeCountry {
		if !isValidCountry(strings.ToUpper(country)) {
			return invalid("conditions.excludeCountry values must be valid ISO 3166-1 alpha-2 codes")
		}
	}

	for _, p := range c.Platform {
		if _, ok := validPlatforms[strings.ToLower(p)]; !ok {
			return invalid("conditions.platform values must be android, ios, or web")
		}
	}

	for _, p := range c.ExcludePlatform {
		if _, ok := validPlatforms[strings.ToLower(p)]; !ok {
			return invalid("conditions.excludePlatform values must be android, ios, or web")
		}
	}

	if (c.DaypartStart != nil) != (c.DaypartEnd != nil) {
		return invalid("conditions.daypartStart and daypartEnd must be both set or both unset")
	}

	if c.DaypartStart != nil {
		h1, m1, ok1 := parseDaypart(*c.DaypartStart)
		h2, m2, ok2 := parseDaypart(*c.DaypartEnd)
		if !ok1 || !ok2 {
			return invalid("conditions.daypartStart and daypartEnd must be in HH:MM format")
		}
		if h1 < 0 || h1 > 23 || m1 < 0 || m1 > 59 || h2 < 0 || h2 > 23 || m2 < 0 || m2 > 59 {
			return invalid("conditions.daypartStart and daypartEnd must be valid times (00:00-23:59)")
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

	if len(c.ExcludeCountry) > 0 {
		countries := make([]string, len(c.ExcludeCountry))
		for i, country := range c.ExcludeCountry {
			countries[i] = strings.ToUpper(strings.TrimSpace(country))
		}
		c.ExcludeCountry = countries
	}

	if len(c.Platform) > 0 {
		platforms := make([]string, len(c.Platform))
		for i, platform := range c.Platform {
			platforms[i] = strings.ToLower(strings.TrimSpace(platform))
		}
		c.Platform = platforms
	}

	if len(c.ExcludePlatform) > 0 {
		platforms := make([]string, len(c.ExcludePlatform))
		for i, platform := range c.ExcludePlatform {
			platforms[i] = strings.ToLower(strings.TrimSpace(platform))
		}
		c.ExcludePlatform = platforms
	}

	return c
}

func parseDaypart(s string) (hours, minutes int, ok bool) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	h, ok1 := parseInt(parts[0])
	m, ok2 := parseInt(parts[1])
	if !ok1 || !ok2 {
		return 0, 0, false
	}
	return h, m, true
}

func parseDaypartMinutes(s string) int {
	h, m, ok := parseDaypart(s)
	if !ok {
		return -1
	}
	return h*60 + m
}

func parseInt(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return 0, false
	}
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}
