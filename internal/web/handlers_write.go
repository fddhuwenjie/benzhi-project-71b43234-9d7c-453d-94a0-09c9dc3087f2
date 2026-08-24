package web

import (
	"net/http"

	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/application"
	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/domain"
)

func (s *Server) CreateApplicationHandler(w http.ResponseWriter, r *http.Request) {
	var command application.CreateDraftCommand
	if err := decodeJSON(w, r, &command); err != nil {
		handleError(w, err)
		return
	}
	view, err := s.service.CreateDraft(command)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, view)
}

func (s *Server) SaveDraftHandler(w http.ResponseWriter, r *http.Request) {
	var command application.SaveDraftCommand
	if err := decodeJSON(w, r, &command); err != nil {
		handleError(w, err)
		return
	}
	view, err := s.service.SaveDraft(r.PathValue("id"), command)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) AssessHandler(w http.ResponseWriter, r *http.Request) {
	var command application.AssessCommand
	if err := decodeJSON(w, r, &command); err != nil {
		handleError(w, err)
		return
	}
	view, err := s.service.Assess(r.PathValue("id"), command)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) SubmitHandler(w http.ResponseWriter, r *http.Request) {
	var command application.SubmitCommand
	if err := decodeJSON(w, r, &command); err != nil {
		handleError(w, err)
		return
	}
	view, err := s.service.Submit(r.PathValue("id"), command)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) WarningDispositionHandler(w http.ResponseWriter, r *http.Request) {
	var command application.WarningDispositionCommand
	if err := decodeJSON(w, r, &command); err != nil {
		handleError(w, err)
		return
	}
	view, err := s.service.SaveWarningDisposition(r.PathValue("id"), command)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) SiteEvidenceHandler(w http.ResponseWriter, r *http.Request) {
	var command application.SiteCommand
	if err := decodeJSON(w, r, &command); err != nil {
		handleError(w, err)
		return
	}
	view, err := s.service.CompleteSiteStrict(r.PathValue("id"), command)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) SiteEvidenceDraftHandler(w http.ResponseWriter, r *http.Request) {
	var command application.SiteDraftCommand
	if err := decodeJSON(w, r, &command); err != nil {
		handleError(w, err)
		return
	}
	view, err := s.service.SaveSiteDraft(r.PathValue("id"), command)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) DeleteSitePhotoHandler(w http.ResponseWriter, r *http.Request) {
	var command application.DeletePhotoCommand
	if err := decodeJSON(w, r, &command); err != nil {
		handleError(w, err)
		return
	}
	view, err := s.service.DeleteSitePhoto(r.PathValue("id"), r.PathValue("photoID"), command)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) ConfirmSiteEvidenceHandler(w http.ResponseWriter, r *http.Request) {
	var command application.ConfirmSiteCommand
	if err := decodeJSON(w, r, &command); err != nil {
		handleError(w, err)
		return
	}
	view, err := s.service.ConfirmSite(r.PathValue("id"), command)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) ReviewHandler(w http.ResponseWriter, r *http.Request) {
	var command application.ReviewCommand
	if err := decodeJSON(w, r, &command); err != nil {
		handleError(w, err)
		return
	}
	if command.Review.Matrix == nil {
		command.Review.Matrix = []domain.ReviewMatrixInput{}
	}
	view, err := s.service.Review(r.PathValue("id"), command)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) RectificationHandler(w http.ResponseWriter, r *http.Request) {
	var command application.RectifyCommand
	if err := decodeJSON(w, r, &command); err != nil {
		handleError(w, err)
		return
	}
	view, err := s.service.Rectify(r.PathValue("id"), command)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) ResubmitHandler(w http.ResponseWriter, r *http.Request) {
	var command application.ResubmitCommand
	if err := decodeJSON(w, r, &command); err != nil {
		handleError(w, err)
		return
	}
	view, err := s.service.Resubmit(r.PathValue("id"), command)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) ArchiveIntegrityHandler(w http.ResponseWriter, r *http.Request) {
	var command application.VerifyArchiveCommand
	if err := decodeJSON(w, r, &command); err != nil {
		handleError(w, err)
		return
	}
	receipt, err := s.service.VerifyArchiveContext(r.Context(), r.PathValue("id"), command)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, receipt)
}
