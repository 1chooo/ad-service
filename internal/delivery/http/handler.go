package httpdelivery

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/1chooo/ad-service/internal/model"
	"github.com/1chooo/ad-service/internal/service"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	svc *service.AdService
}

func NewHandler(svc *service.AdService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/health", h.health)
	r.MethodFunc(http.MethodGet, "/api/v1/ad", h.listAds)
	r.MethodFunc(http.MethodPost, "/api/v1/ad", h.createAd)
	r.MethodFunc(http.MethodPut, "/api/v1/ad", methodNotAllowed)
	r.MethodFunc(http.MethodPatch, "/api/v1/ad", methodNotAllowed)
	r.MethodFunc(http.MethodDelete, "/api/v1/ad", methodNotAllowed)
	return r
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

	ad, err := h.svc.CreateAd(r.Context(), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, ad)
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
