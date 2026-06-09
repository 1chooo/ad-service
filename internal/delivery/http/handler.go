package httpdelivery

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/1chooo/ad-service/internal/model"
	"github.com/1chooo/ad-service/internal/service"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	svc         *service.AdService
	rateLimiter *RateLimiter
	idempotent  *IdempotencyStore
}

func NewHandler(svc *service.AdService) *Handler {
	return &Handler{
		svc:         svc,
		rateLimiter: NewRateLimiter(100, time.Minute),
		idempotent:  NewIdempotencyStore(5 * time.Minute),
	}
}

func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/health", h.health)
	r.MethodFunc(http.MethodGet, "/api/v1/ad", h.listAds)
	r.With(h.adminRateLimit).MethodFunc(http.MethodPost, "/api/v1/ad", h.createAd)
	r.With(h.adminRateLimit).MethodFunc(http.MethodPost, "/api/v1/ads", h.bulkCreateAds)
	r.MethodFunc(http.MethodPut, "/api/v1/ad", methodNotAllowed)
	r.MethodFunc(http.MethodPatch, "/api/v1/ad", methodNotAllowed)
	r.MethodFunc(http.MethodDelete, "/api/v1/ad", methodNotAllowed)
	return r
}

func (h *Handler) adminRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r)
		if !h.rateLimiter.Allow(ip) {
			writeError(w, http.StatusTooManyRequests, "RATE_LIMITED", "too many requests, try again later")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func (h *Handler) createAd(w http.ResponseWriter, r *http.Request) {
	var req model.CreateAdRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, model.ErrCodeInvalidArgument, "request body must be valid JSON")
		return
	}

	if key := r.Header.Get("Idempotency-Key"); key != "" {
		if cached, ok := h.idempotent.Get(key); ok {
			writeJSON(w, http.StatusOK, cached)
			return
		}

		ad, err := h.svc.CreateAd(r.Context(), req)
		if err != nil {
			writeServiceError(w, err)
			return
		}

		h.idempotent.Set(key, ad)
		writeJSON(w, http.StatusCreated, ad)
		return
	}

	ad, err := h.svc.CreateAd(r.Context(), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, ad)
}

func (h *Handler) bulkCreateAds(w http.ResponseWriter, r *http.Request) {
	var req model.BulkCreateAdRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, model.ErrCodeInvalidArgument, "request body must be valid JSON")
		return
	}

	resp, err := h.svc.BulkCreateAds(r.Context(), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) listAds(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	offset := model.DefaultOffset
	if raw := q.Get("offset"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, model.ErrCodeInvalidArgument, "offset must be a non-negative integer")
			return
		}
		offset = parsed
	}

	limit := model.DefaultLimit
	if raw := q.Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, model.ErrCodeInvalidArgument, "limit must be between 1 and 100")
			return
		}
		limit = parsed
	}

	var age *int
	if raw := q.Get("age"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, model.ErrCodeInvalidArgument, "age must be between 1 and 100")
			return
		}
		age = &parsed
	}

	var gender, country, platform *string
	if raw := strings.TrimSpace(q.Get("gender")); raw != "" {
		gender = &raw
	}
	if raw := strings.TrimSpace(q.Get("country")); raw != "" {
		country = &raw
	}
	if raw := strings.TrimSpace(q.Get("platform")); raw != "" {
		platform = &raw
	}

	query, err := model.ValidateListQuery(offset, limit, age, gender, country, platform)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	resp, err := h.svc.ListAds(r.Context(), query)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func methodNotAllowed(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
}

type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeServiceError(w http.ResponseWriter, err error) {
	var validationErr *model.ValidationError
	if errors.As(err, &validationErr) {
		writeError(w, http.StatusBadRequest, validationErr.Code, validationErr.Message)
		return
	}
	writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "unexpected server error")
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{
		Error: errorBody{
			Code:    code,
			Message: message,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func extractIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		parts := strings.SplitN(fwd, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return strings.TrimSpace(realIP)
	}
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}

type RateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*rateEntry
	limit    int
	window   time.Duration
}

type rateEntry struct {
	count   int
	resetAt time.Time
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		visitors: make(map[string]*rateEntry),
		limit:    limit,
		window:   window,
	}
}

func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, ok := rl.visitors[key]

	if !ok || now.After(entry.resetAt) {
		rl.visitors[key] = &rateEntry{
			count:   1,
			resetAt: now.Add(rl.window),
		}
		return true
	}

	if entry.count >= rl.limit {
		return false
	}

	entry.count++
	return true
}

type IdempotencyStore struct {
	mu   sync.Mutex
	data map[string]*idempotentEntry
	ttl  time.Duration
}

type idempotentEntry struct {
	response  any
	expiresAt time.Time
}

func NewIdempotencyStore(ttl time.Duration) *IdempotencyStore {
	s := &IdempotencyStore{
		data: make(map[string]*idempotentEntry),
		ttl:  ttl,
	}
	go s.cleanup()
	return s
}

func (s *IdempotencyStore) Get(key string) (any, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.data[key]
	if !ok || time.Now().After(entry.expiresAt) {
		if ok {
			delete(s.data, key)
		}
		return nil, false
	}

	return entry.response, true
}

func (s *IdempotencyStore) Set(key string, response any) {
	s.mu.Lock()
	defer s.mu.Unlock()

	h := sha256.Sum256([]byte(fmt.Sprintf("%v", response)))
	_ = h

	s.data[key] = &idempotentEntry{
		response:  response,
		expiresAt: time.Now().Add(s.ttl),
	}
}

func (s *IdempotencyStore) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for key, entry := range s.data {
			if now.After(entry.expiresAt) {
				delete(s.data, key)
			}
		}
		s.mu.Unlock()
	}
}
