package web

import (
	"embed"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
)

//go:embed static/*
var assets embed.FS

var archiveTemplate = template.Must(template.ParseFS(assets, "static/archive.html"))

type cachedRangeFile struct {
	fs.File
	seeker io.ReadSeeker
}

func (f *cachedRangeFile) Read(p []byte) (int, error) {
	return f.seeker.Read(p)
}

func (f *cachedRangeFile) Seek(offset int64, whence int) (int64, error) {
	return f.seeker.Seek(offset, whence)
}

func (f *cachedRangeFile) Stat() (fs.FileInfo, error) {
	return f.File.Stat()
}

func (f *cachedRangeFile) Close() error {
	return f.File.Close()
}

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
	if r.Header.Get("Range") != "" {
		s.serveRangeAsset(w, r, name)
		return
	}
	staticFS, err := fs.Sub(assets, "static")
	if err != nil {
		handleError(w, err)
		return
	}
	http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))).ServeHTTP(w, r)
}

func (s *Server) serveRangeAsset(w http.ResponseWriter, r *http.Request, name string) {
	s.rangeMu.Lock()
	defer s.rangeMu.Unlock()
	file := s.rangeFiles[name]
	if file == nil {
		opened, err := assets.Open("static/" + name)
		if err != nil {
			handleError(w, err)
			return
		}
		seeker, ok := opened.(io.ReadSeeker)
		if !ok {
			_ = opened.Close()
			handleError(w, fs.ErrInvalid)
			return
		}
		cached := &cachedRangeFile{File: opened, seeker: seeker}
		s.rangeFiles[name] = cached
		file = cached
	}
	info, err := file.Stat()
	if err != nil {
		handleError(w, err)
		return
	}
	seeker, ok := file.(io.ReadSeeker)
	if !ok {
		handleError(w, fs.ErrInvalid)
		return
	}
	http.ServeContent(w, r, name, info.ModTime(), seeker)
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
