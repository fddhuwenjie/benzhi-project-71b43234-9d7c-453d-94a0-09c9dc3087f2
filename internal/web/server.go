package web

import (
	"io/fs"
	"log/slog"
	"net/http"
	"sync"

	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/application"
)

const maxRequestBody = 1 << 20

type Server struct {
	service    *application.Service
	logger     *slog.Logger
	mux        *http.ServeMux
	rangeMu    sync.Mutex
	rangeFiles map[string]fs.File
}

func NewServer(service *application.Service, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	s := &Server{service: service, logger: logger, mux: http.NewServeMux(), rangeFiles: make(map[string]fs.File)}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.securityHeaders(s.recoverPanic(s.mux))
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /{$}", s.WorkbenchHandler)
	s.mux.HandleFunc("GET /archive/{id}", s.ArchivePageHandler)
	s.mux.HandleFunc("GET /static/{name}", s.StaticHandler)
	s.mux.HandleFunc("GET /api/health", s.HealthHandler)
	s.mux.HandleFunc("GET /api/applications", s.ListApplicationsHandler)
	s.mux.HandleFunc("POST /api/applications", s.CreateApplicationHandler)
	s.mux.HandleFunc("GET /api/applications/{id}", s.GetApplicationHandler)
	s.mux.HandleFunc("PUT /api/applications/{id}/draft", s.SaveDraftHandler)
	s.mux.HandleFunc("POST /api/applications/{id}/assess", s.AssessHandler)
	s.mux.HandleFunc("POST /api/applications/{id}/submit", s.SubmitHandler)
	s.mux.HandleFunc("PUT /api/applications/{id}/warning-dispositions", s.WarningDispositionHandler)
	s.mux.HandleFunc("POST /api/applications/{id}/site-evidence", s.SiteEvidenceHandler)
	s.mux.HandleFunc("PUT /api/applications/{id}/site-evidence/draft", s.SiteEvidenceDraftHandler)
	s.mux.HandleFunc("DELETE /api/applications/{id}/site-evidence/photos/{photoID}", s.DeleteSitePhotoHandler)
	s.mux.HandleFunc("POST /api/applications/{id}/site-evidence/confirm", s.ConfirmSiteEvidenceHandler)
	s.mux.HandleFunc("POST /api/applications/{id}/review", s.ReviewHandler)
	s.mux.HandleFunc("POST /api/applications/{id}/rectifications", s.RectificationHandler)
	s.mux.HandleFunc("POST /api/applications/{id}/resubmit", s.ResubmitHandler)
	s.mux.HandleFunc("POST /api/applications/{id}/archive-integrity", s.ArchiveIntegrityHandler)
}
