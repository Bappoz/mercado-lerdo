package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"nexora/internal/store"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
	minSearchLen    = 2
	maxSearchLen    = 64
)

func (s *Server) listProducts(w http.ResponseWriter, r *http.Request) {
	limit, ok := intParam(w, r, "limit", defaultPageSize, 1, maxPageSize)
	if !ok {
		return
	}
	page, ok := intParam(w, r, "page", 1, 1, 10_000)
	if !ok {
		return
	}
	products, err := s.store.ListProducts(r.Context(), limit, (page-1)*limit)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"page": page, "products": products})
}

func (s *Server) getProduct(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "id de produto inválido")
		return
	}
	p, err := s.store.GetProduct(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "produto não encontrado")
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if n := utf8.RuneCountInString(q); n < minSearchLen || n > maxSearchLen {
		writeError(w, http.StatusBadRequest, "q precisa ter de 2 a 64 caracteres")
		return
	}
	limit, ok := intParam(w, r, "limit", defaultPageSize, 1, maxPageSize)
	if !ok {
		return
	}
	products, err := s.store.SearchProducts(r.Context(), q, limit)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"query": q, "products": products})
}

// intParam lê um parâmetro inteiro opcional da query, dentro de [lo, hi].
// Com valor inválido, responde 400 e devolve false.
func intParam(w http.ResponseWriter, r *http.Request, name string, def, lo, hi int) (int, bool) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return def, true
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < lo || v > hi {
		writeError(w, http.StatusBadRequest, "parâmetro inválido: "+name)
		return 0, false
	}
	return v, true
}

func (s *Server) internalError(w http.ResponseWriter, r *http.Request, err error) {
	s.log.Error("requisição falhou", "path", r.URL.Path, "err", err)
	writeError(w, http.StatusInternalServerError, "erro interno")
}
