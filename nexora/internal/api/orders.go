package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"nexora/internal/auth"
	"nexora/internal/store"
)

const sessionTTL = 24 * time.Hour

type ctxKey int

const userIDKey ctxKey = iota

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	creds, err := s.store.CredentialsByEmail(r.Context(), strings.ToLower(strings.TrimSpace(req.Email)))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	ok, err := auth.VerifyPassword(req.Password, creds.PasswordHash)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	token, err := auth.NewToken()
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	if err := s.store.CreateSession(r.Context(), token, creds.UserID, sessionTTL); err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "expires_in": int(sessionTTL.Seconds())})
}

func (s *Server) requireUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !found || token == "" {
			writeError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}
		userID, err := s.store.UserBySession(r.Context(), token)
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "invalid or expired session")
			return
		}
		if err != nil {
			s.internalError(w, r, err)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), userIDKey, userID)))
	}
}

type checkoutRequest struct {
	OrderRef string       `json:"order_ref"`
	Items    []store.Item `json:"items"`
}

func (s *Server) checkout(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Idempotency-Key") == "" {
		writeError(w, http.StatusBadRequest, "missing Idempotency-Key header")
		return
	}
	var req checkoutRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if msg := validateCheckout(req); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	userID, _ := r.Context().Value(userIDKey).(int64)
	order, err := s.store.PlaceOrder(r.Context(), userID, req.OrderRef, req.Items, s.payment.Charge)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusUnprocessableEntity, "unknown product")
	case errors.Is(err, store.ErrOutOfStock):
		writeError(w, http.StatusConflict, "out of stock")
	case err != nil:
		s.internalError(w, r, err)
	default:
		writeJSON(w, http.StatusCreated, order)
	}
}

func validateCheckout(req checkoutRequest) string {
	if l := len(req.OrderRef); l < 8 || l > 64 {
		return "order_ref must have between 8 and 64 characters"
	}
	if len(req.Items) == 0 || len(req.Items) > 20 {
		return "items must have between 1 and 20 entries"
	}
	seen := make(map[int64]bool, len(req.Items))
	for _, it := range req.Items {
		if it.ProductID <= 0 || it.Quantity <= 0 || it.Quantity > 10 {
			return "each item needs a valid product_id and a quantity between 1 and 10"
		}
		if seen[it.ProductID] {
			return "duplicate product_id in items"
		}
		seen[it.ProductID] = true
	}
	return ""
}

func (s *Server) listOrders(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(userIDKey).(int64)
	orders, err := s.store.OrdersByUser(r.Context(), userID, 50)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"orders": orders})
}
