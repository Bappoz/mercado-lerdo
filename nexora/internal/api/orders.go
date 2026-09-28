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
		writeError(w, http.StatusBadRequest, "corpo JSON inválido")
		return
	}
	creds, err := s.store.CredentialsByEmail(r.Context(), strings.ToLower(strings.TrimSpace(req.Email)))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusUnauthorized, "e-mail ou senha inválidos")
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
		writeError(w, http.StatusUnauthorized, "e-mail ou senha inválidos")
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
			writeError(w, http.StatusUnauthorized, "token Bearer ausente")
			return
		}
		userID, err := s.store.UserBySession(r.Context(), token)
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "sessão inválida ou expirada")
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
		writeError(w, http.StatusBadRequest, "cabeçalho Idempotency-Key ausente")
		return
	}
	var req checkoutRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "corpo JSON inválido")
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
		writeError(w, http.StatusUnprocessableEntity, "produto desconhecido")
	case errors.Is(err, store.ErrOutOfStock):
		writeError(w, http.StatusConflict, "estoque insuficiente")
	case err != nil:
		s.internalError(w, r, err)
	default:
		writeJSON(w, http.StatusCreated, order)
	}
}

func validateCheckout(req checkoutRequest) string {
	if l := len(req.OrderRef); l < 8 || l > 64 {
		return "order_ref precisa ter de 8 a 64 caracteres"
	}
	if len(req.Items) == 0 || len(req.Items) > 20 {
		return "items precisa ter de 1 a 20 itens"
	}
	seen := make(map[int64]bool, len(req.Items))
	for _, it := range req.Items {
		if it.ProductID <= 0 || it.Quantity <= 0 || it.Quantity > 10 {
			return "cada item precisa de um product_id válido e de quantity entre 1 e 10"
		}
		if seen[it.ProductID] {
			return "product_id repetido em items"
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
