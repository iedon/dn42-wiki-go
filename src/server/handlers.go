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

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

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
		case errors.Is(err, site.ErrReservedPath):
			writeError(w, http.StatusBadRequest, "The specified path is reserved and cannot be used")
		case errors.Is(err, site.ErrInvalidPath):
			writeError(w, http.StatusBadRequest, err.Error())
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
		case errors.Is(err, site.ErrReservedPath):
			writeError(w, http.StatusBadRequest, "The specified path is reserved and cannot be used")
		case errors.Is(err, site.ErrInvalidPath):
			writeError(w, http.StatusBadRequest, err.Error())
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

	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "Home"
	}

	html, err := s.svc.RenderFullPage(r.Context(), path)
	if err != nil {
		switch {
		case errors.Is(err, site.ErrInvalidPath), errors.Is(err, os.ErrNotExist):
			notFound, renderErr := s.svc.RenderNotFoundPage(r.Context(), r.URL.Path)
			if renderErr != nil {
				writeError(w, http.StatusInternalServerError, renderErr.Error())
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write(notFound)
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(html)
}

func isSafeRevision(ref string) bool {
	return safeRevisionPattern.MatchString(strings.TrimSpace(ref))
}

func (s *Server) clientRemoteAddr(r *http.Request) string {
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
