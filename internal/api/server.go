package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"detection_engine/internal/config"
	"detection_engine/internal/engine"
	"detection_engine/internal/ingest"
	"detection_engine/internal/stack"
	"detection_engine/internal/workspace"
)

type Server struct {
	cfg    config.Config
	engine *engine.Engine
	log    *slog.Logger
}

func New(cfg config.Config, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	return &Server{cfg: cfg, engine: engine.New(cfg), log: log}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /readyz", s.health)
	mux.HandleFunc("POST /v1/detect/github", s.detectGitHub)
	mux.HandleFunc("POST /v1/detect/zip", s.detectZip)
	if s.cfg.AllowLocalPath {
		mux.HandleFunc("POST /v1/detect/path", s.detectPath)
	}
	return withCommon(mux)
}

func withCommon(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		origin := r.Header.Get("Origin")
		if origin == "http://localhost:3000" || origin == "http://127.0.0.1:3000" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-GitHub-Token")
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type githubBody struct {
	URL string `json:"url"`
	Ref string `json:"ref"`
}

type pathBody struct {
	Path string `json:"path"`
}

func (s *Server) detectGitHub(w http.ResponseWriter, r *http.Request) {
	var body githubBody
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_request", "JSON body with url is required")
		return
	}
	ref, err := ingest.ParseGitHub(body.URL, body.Ref)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_url", "Use owner/repo or a github.com URL")
		return
	}

	space, err := workspace.New(s.cfg.WorkDir)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "workspace", "Could not create isolated workspace")
		return
	}
	defer func() { _ = space.Close() }()

	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.CloneTimeout+s.cfg.DetectTimeout)
	defer cancel()

	token := firstNonEmpty(r.Header.Get("X-GitHub-Token"), s.cfg.GitHubToken)
	source, err := ingest.CloneGitHub(ctx, s.cfg, space.Repo, ref, token)
	if err != nil {
		s.log.Warn("clone failed", "err", err, "repo", ref.Owner+"/"+ref.Name)
		status, code, msg := cloneStatus(err)
		writeErr(w, status, code, msg)
		return
	}

	result, err := s.engine.DetectDir(ctx, space.ID, space.Repo, source)
	if err != nil {
		s.log.Error("detect failed", "err", err, "id", space.ID)
		writeErr(w, http.StatusInternalServerError, "detect_failed", "Detection failed")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) detectZip(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxUploadBytes)
	if err := r.ParseMultipartForm(s.cfg.MaxUploadBytes); err != nil {
		writeErr(w, http.StatusRequestEntityTooLarge, "too_large", "Zip exceeds upload limit")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_zip", "Multipart field file is required")
		return
	}
	defer file.Close()

	if !strings.HasSuffix(strings.ToLower(header.Filename), ".zip") {
		writeErr(w, http.StatusBadRequest, "invalid_zip", "File must be a .zip")
		return
	}

	tmp, err := os.CreateTemp(s.cfg.WorkDir, "stack-upload-*.zip")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "workspace", "Could not store upload")
		return
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	if _, err := io.Copy(tmp, file); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_zip", "Could not read zip upload")
		return
	}
	info, err := tmp.Stat()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "workspace", "Could not stat upload")
		return
	}
	if _, err := tmp.Seek(0, 0); err != nil {
		writeErr(w, http.StatusInternalServerError, "workspace", "Could not rewind upload")
		return
	}

	space, err := workspace.New(s.cfg.WorkDir)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "workspace", "Could not create isolated workspace")
		return
	}
	defer func() { _ = space.Close() }()

	source, err := ingest.ExtractZip(tmp, info.Size(), space.Repo, s.cfg, header.Filename)
	if err != nil {
		status, code, msg := zipStatus(err)
		writeErr(w, status, code, msg)
		return
	}

	result, err := s.engine.DetectDir(r.Context(), space.ID, space.Repo, source)
	if err != nil {
		s.log.Error("detect failed", "err", err, "id", space.ID)
		writeErr(w, http.StatusInternalServerError, "detect_failed", "Detection failed")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) detectPath(w http.ResponseWriter, r *http.Request) {
	var body pathBody
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil || strings.TrimSpace(body.Path) == "" {
		writeErr(w, http.StatusBadRequest, "invalid_request", "JSON body with path is required")
		return
	}
	result, err := s.engine.DetectDir(r.Context(), "local", body.Path, stack.Source{Type: stack.SourceDir, Name: body.Path})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "detect_failed", "Detection failed")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func cloneStatus(err error) (int, string, string) {
	switch {
	case errors.Is(err, ingest.ErrInvalidGitHubURL):
		return http.StatusBadRequest, "invalid_url", "Use owner/repo or a github.com URL"
	case errors.Is(err, ingest.ErrGitMissing):
		return http.StatusServiceUnavailable, "unavailable", "git is required on the detection host"
	case errors.Is(err, ingest.ErrRepoTooLarge):
		return http.StatusRequestEntityTooLarge, "too_large", "Repository exceeds size limit"
	case errors.Is(err, context.DeadlineExceeded):
		return http.StatusGatewayTimeout, "timeout", "Clone timed out"
	default:
		return http.StatusBadGateway, "clone_failed", "Could not clone the GitHub repository"
	}
}

func zipStatus(err error) (int, string, string) {
	switch {
	case errors.Is(err, ingest.ErrZipSlip), errors.Is(err, ingest.ErrZipSymlink):
		return http.StatusBadRequest, "invalid_zip", "Zip failed security validation"
	case errors.Is(err, ingest.ErrZipTooLarge):
		return http.StatusRequestEntityTooLarge, "too_large", "Zip exceeds size limit"
	default:
		return http.StatusBadRequest, "invalid_zip", "Could not extract zip"
	}
}

func writeErr(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, stack.ErrorBody{Error: stack.APIError{Code: code, Message: message}})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func NewHTTPServer(cfg config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       2 * time.Minute,
		WriteTimeout:      3 * time.Minute,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 16,
	}
}
