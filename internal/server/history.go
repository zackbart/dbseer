package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/zackbart/dbseer/internal/safety"
)

// historyResponse is the JSON shape for GET /api/history.
type historyResponse struct {
	Entries []safety.Entry `json:"entries"`
}

// handleHistory handles GET /api/history.
func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	limit := 50
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeError(w, 400, "invalid_request", "invalid limit", nil)
			return
		}
		if n > 1000 {
			n = 1000
		}
		limit = n
	}

	var sinceTS time.Time
	if v := q.Get("since"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, 400, "invalid_request", "since must be RFC3339", nil)
			return
		}
		sinceTS = t
	}

	tableFilter := q.Get("table")

	if s.cfg.AuditLog == nil {
		writeJSON(w, 200, historyResponse{Entries: []safety.Entry{}})
		return
	}

	entries, err := s.cfg.AuditLog.Read(limit, sinceTS, tableFilter)
	if err != nil {
		writeError(w, 500, "internal", "failed to read audit log", nil)
		return
	}

	if entries == nil {
		entries = []safety.Entry{}
	}

	writeJSON(w, 200, historyResponse{Entries: entries})
}
