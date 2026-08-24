package web

import (
	"net/http"
	"time"
)

func (s *Server) HealthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "time": time.Now().UTC()})
}

func (s *Server) ListApplicationsHandler(w http.ResponseWriter, _ *http.Request) {
	applications, err := s.service.List()
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"applications": applications})
}

func (s *Server) GetApplicationHandler(w http.ResponseWriter, r *http.Request) {
	view, err := s.service.Get(r.PathValue("id"))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}
