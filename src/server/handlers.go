package server

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/iedon/dn42-wiki-go/site"
)

var safeRevisionPattern = regexp.MustCompile(`^[0-9A-Fa-f]{4,64}$`)

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	path := r.URL.Query().Get("path")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize <= 0 {
		pageSize = 25
	}

	commits, hasMore, err := s.svc.History(r.Context(), path, page, pageSize)
	if err != nil {
		switch {
		case errors.Is(err, site.ErrInvalidPath):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, site.ErrForbiddenRoute):
			writeError(w, http.StatusForbidden, "requested path is restricted")
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": commits, "hasMore": hasMore})
}

func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	path := r.URL.Query().Get("path")
	from := strings.TrimSpace(r.URL.Query().Get("from"))
	to := strings.TrimSpace(r.URL.Query().Get("to"))
	if from == "" || to == "" {
		writeError(w, http.StatusBadRequest, "from and to are required")
		return
	}
	if !isSafeRevision(from) || !isSafeRevision(to) {
		writeError(w, http.StatusBadRequest, "invalid revision reference")
		return
	}
	diff, err := s.svc.Diff(r.Context(), path, from, to)
	if err != nil {
		switch {
		case errors.Is(err, site.ErrInvalidPath):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, site.ErrForbiddenRoute):
			writeError(w, http.StatusForbidden, "requested path is restricted")
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"diff": diff})
}

func (s *Server) handleDocument(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	path := r.URL.Query().Get("path")
	content, err := s.svc.LoadRaw(path)
	if err != nil {
		switch {
		case errors.Is(err, site.ErrInvalidPath):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, os.ErrNotExist):
			writeError(w, http.StatusNotFound, "document not found")
		case errors.Is(err, site.ErrForbiddenRoute):
			writeError(w, http.StatusForbidden, "requested path is restricted")
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": path, "content": string(content)})
}

func (s *Server) handleSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.cfg.Editable {
		writeError(w, http.StatusForbidden, "editing disabled")
		return
	}
	var payload struct {
		Path    string `json:"path"`
		Content string `json:"content"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	remote := s.clientRemoteAddr(r)
	if err := s.svc.SavePage(r.Context(), payload.Path, []byte(payload.Content), payload.Message, remote); err != nil {
		switch {
		case errors.Is(err, site.ErrRepositoryBehind):
			writeError(w, http.StatusConflict, "remote repository has newer revisions; please save current work and reload")
		case errors.Is(err, site.ErrReservedPath):
			writeError(w, http.StatusBadRequest, "The specified path is reserved and cannot be used")
		case errors.Is(err, site.ErrInvalidPath):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, site.ErrForbiddenRoute):
			writeError(w, http.StatusForbidden, "requested path is restricted")
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func (s *Server) handleRename(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.cfg.Editable {
		writeError(w, http.StatusForbidden, "editing disabled")
		return
	}
	var payload struct {
		OldPath string `json:"oldPath"`
		NewPath string `json:"newPath"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if payload.NewPath == "" {
		writeError(w, http.StatusBadRequest, "newPath required")
		return
	}
	remote := s.clientRemoteAddr(r)
	if err := s.svc.RenamePage(r.Context(), payload.OldPath, payload.NewPath, remote); err != nil {
		switch {
		case errors.Is(err, site.ErrRepositoryBehind):
			writeError(w, http.StatusConflict, "remote repository has newer revisions; please reload")
		case errors.Is(err, site.ErrReservedPath):
			writeError(w, http.StatusBadRequest, "The specified path is reserved and cannot be used")
		case errors.Is(err, site.ErrInvalidPath):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, site.ErrForbiddenRoute):
			writeError(w, http.StatusForbidden, "requested path is restricted")
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "renamed"})
}

func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var payload struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	rendered, err := s.svc.RenderPreview([]byte(payload.Content))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"html": string(rendered.HTML), "headings": rendered.Headings})
}

func (s *Server) handleSearchIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	payload := s.svc.SearchIndex()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if len(payload) == 0 {
		payload = []byte(`{}`)
	}
	_, _ = w.Write(payload)
}

func (s *Server) handlePage(w http.ResponseWriter, r *http.Request) {
	if s.tryStatic(w, r) {
		return
	}
	if s.redirectCanonical(w, r) {
		return
	}
	if err := s.svc.EnsureRequestAccessible(r.URL.Path); err != nil {
		switch {
		case errors.Is(err, site.ErrForbiddenRoute):
			s.serveForbidden(w, r)
		case errors.Is(err, site.ErrInvalidPath):
			s.serveNotFound(w, r)
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	staticPath, err := s.svc.StaticDocumentPath(r.URL.Path)
	if err != nil {
		s.serveNotFound(w, r)
		return
	}
	if !isWithin(s.cfg.OutputDir, staticPath) {
		s.serveNotFound(w, r)
		return
	}
	info, err := os.Stat(staticPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			s.serveNotFound(w, r)
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if info.IsDir() {
		s.serveNotFound(w, r)
		return
	}
	http.ServeFile(w, r, staticPath)
}

func (s *Server) serveNotFound(w http.ResponseWriter, r *http.Request) {
	if page, err := s.svc.RenderNotFoundPage(r.Context(), r.URL.Path); err == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write(page)
		return
	}
	// fallback
	writeError(w, http.StatusNotFound, "not found")
}

func (s *Server) serveForbidden(w http.ResponseWriter, r *http.Request) {
	if page, err := s.svc.RenderForbiddenPage(r.Context(), r.URL.Path); err == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write(page)
		return
	}
	// fallback
	writeError(w, http.StatusForbidden, "forbidden")
}

func (s *Server) redirectCanonical(w http.ResponseWriter, r *http.Request) bool {
	target, alias, redirect, err := s.svc.CanonicalRedirect(r.URL.Path)
	if err != nil {
		if errors.Is(err, site.ErrInvalidPath) {
			return false
		}
		s.logger.Error("canonical redirect", "error", err, "path", r.URL.Path)
		return false
	}
	if !redirect {
		return false
	}
	if raw := r.URL.RawQuery; raw != "" {
		target += "?" + raw
	}
	status := http.StatusMovedPermanently
	if alias {
		status = http.StatusFound
	}
	http.Redirect(w, r, target, status)
	return true
}

func isSafeRevision(ref string) bool {
	return safeRevisionPattern.MatchString(strings.TrimSpace(ref))
}

func (s *Server) clientRemoteAddr(r *http.Request) string {
	if !s.trustForwardHeaders {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err == nil && host != "" {
			return host
		}
		return strings.TrimSpace(r.RemoteAddr)
	}

	addr, chain := s.cfg.RemoteAddrFromRequest(r)
	if addr.IsValid() {
		return addr.String()
	}
	if len(chain) > 0 {
		if last := chain[len(chain)-1]; last.IsValid() {
			return last.String()
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
