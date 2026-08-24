package web

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"path/filepath"
)

//go:embed static/*
var assets embed.FS

var archiveTemplate = template.Must(template.ParseFS(assets, "static/archive.html"))

func (s *Server) WorkbenchHandler(w http.ResponseWriter, _ *http.Request) {
	content, err := assets.ReadFile("static/index.html")
	if err != nil {
		handleError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(content)
}

func (s *Server) StaticHandler(w http.ResponseWriter, r *http.Request) {
	name := filepath.Base(r.PathValue("name"))
	if name != r.PathValue("name") || name == "." {
		http.NotFound(w, r)
		return
	}
	staticFS, err := fs.Sub(assets, "static")
	if err != nil {
		handleError(w, err)
		return
	}
	http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))).ServeHTTP(w, r)
}

func (s *Server) ArchivePageHandler(w http.ResponseWriter, r *http.Request) {
	view, err := s.service.Get(r.PathValue("id"))
	if err != nil {
		handleError(w, err)
		return
	}
	if view.Application.Archive == nil {
		writeError(w, http.StatusConflict, "archive_unavailable", "该申请尚未归档", "")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := archiveTemplate.Execute(w, view); err != nil {
		s.logger.Error("渲染归档页面失败", "error", err)
	}
}
