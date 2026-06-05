package handler

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ReadinessHandler struct {
	db *pgxpool.Pool
}

func NewReadinessHandler(db *pgxpool.Pool) *ReadinessHandler {
	return &ReadinessHandler{db: db}
}

func (h *ReadinessHandler) Ready(w http.ResponseWriter, r *http.Request) {
	if err := h.db.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "unavailable",
			"reason": "database unreachable",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
