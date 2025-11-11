package server

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/iedon/dn42-wiki-go/config"
	"github.com/iedon/dn42-wiki-go/site"
)

// Server ties HTTP handlers to the site service.
type Server struct {
	cfg                 *config.Config
	svc                 *site.Service
	logger              *slog.Logger
	mux                 *http.ServeMux
	trustForwardHeaders bool
	serverHeader        string
}

// New constructs a server instance.
func New(cfg *config.Config, svc *site.Service, logger *slog.Logger, serverHeader string) *Server {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}
	srv := &Server{cfg: cfg, svc: svc, logger: logger, mux: http.NewServeMux(), trustForwardHeaders: true, serverHeader: strings.TrimSpace(serverHeader)}
	srv.routes()
	return srv
}

// Start launches the HTTP server and optional webhook listener.

func (s *Server) Start(ctx context.Context) error {
	// Build static pages on startup
	if err := s.svc.BuildStatic(ctx); err != nil {
		s.logger.Warn("static build", "error", err)
	}

	listener, err := s.listen(s.cfg.Listen)
	if err != nil {
		return err
	}
	if listener != nil {
		if addr := listener.Addr(); addr != nil && addr.Network() == "unix" {
			s.trustForwardHeaders = false
		} else {
			s.trustForwardHeaders = true
		}
	}

	server := &http.Server{
		Handler:      s.withServerHeader(s.logRequests(s.mux)),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	shutdownDone := make(chan struct{})
	go func() {
		<-ctx.Done()
		ctxShutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(ctxShutdown)
		close(shutdownDone)
	}()

	if s.cfg.WebhookEnabled {
		go s.runWebhook(ctx)
	}

	var serveErr error
	if s.cfg.EnableTLS {
		serveErr = server.ServeTLS(listener, s.cfg.TLSCert, s.cfg.TLSKey)
	} else {
		serveErr = server.Serve(listener)
	}

	if errors.Is(serveErr, http.ErrServerClosed) {
		<-shutdownDone
		return nil
	}
	return serveErr
}

func (s *Server) routes() {
	s.mux.HandleFunc("/api/history", s.handleHistory)
	s.mux.HandleFunc("/api/diff", s.handleDiff)
	s.mux.HandleFunc("/api/document", s.handleDocument)
	s.mux.HandleFunc("/api/save", s.handleSave)
	s.mux.HandleFunc("/api/rename", s.handleRename)
	s.mux.HandleFunc("/api/preview", s.handlePreview)
	s.mux.HandleFunc("/search-index.json", s.handleSearchIndex)
	s.mux.HandleFunc("/", s.handlePage)
}

func (s *Server) listen(address string) (net.Listener, error) {
	if listener, ok, err := s.systemdListener(); err != nil {
		return nil, err
	} else if ok {
		return listener, nil
	}
	if after, ok := strings.CutPrefix(address, "unix:"); ok {
		path := after
		_ = os.Remove(path)
		return net.Listen("unix", path)
	}
	return net.Listen("tcp", address)
}

func (s *Server) systemdListener() (net.Listener, bool, error) {
	pidEnv := strings.TrimSpace(os.Getenv("LISTEN_PID"))
	if pidEnv == "" {
		return nil, false, nil
	}
	pid, err := strconv.Atoi(pidEnv)
	if err != nil || pid != os.Getpid() {
		return nil, false, nil
	}
	fdsEnv := strings.TrimSpace(os.Getenv("LISTEN_FDS"))
	if fdsEnv == "" {
		return nil, false, nil
	}
	fds, err := strconv.Atoi(fdsEnv)
	if err != nil {
		return nil, false, fmt.Errorf("systemd listener: invalid LISTEN_FDS: %w", err)
	}
	if fds <= 0 {
		return nil, false, nil
	}
	const sdListenFdsStart = 3
	file := os.NewFile(uintptr(sdListenFdsStart), fmt.Sprintf("systemd-fd-%d", sdListenFdsStart))
	if file == nil {
		return nil, false, fmt.Errorf("systemd listener: failed to access fd")
	}
	listener, err := net.FileListener(file)
	_ = file.Close()
	if err != nil {
		return nil, false, fmt.Errorf("systemd listener: %w", err)
	}
	_ = os.Unsetenv("LISTEN_PID")
	_ = os.Unsetenv("LISTEN_FDS")
	_ = os.Unsetenv("LISTEN_FDNAMES")
	return listener, true, nil
}

func (s *Server) withServerHeader(next http.Handler) http.Handler {
	if s.serverHeader == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", s.serverHeader)
		next.ServeHTTP(w, r)
	})
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		s.logger.Info("http", "method", r.Method, "path", r.URL.Path, "status", rw.status, "duration", time.Since(start))
	})
}

func (s *Server) runWebhook(ctx context.Context) {
	listener, err := s.listen(s.cfg.WebhookListen)
	if err != nil {
		s.logger.Error("webhook listen", "error", err)
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/webhook/pull", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if !s.authorizeWebhook(r) {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if err := s.svc.Pull(r.Context()); err != nil {
			s.logger.Error("webhook pull", "error", err)
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "synced"})
	})
	mux.HandleFunc("/webhook/push", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if !s.authorizeWebhook(r) {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if err := s.svc.Push(r.Context()); err != nil {
			s.logger.Error("webhook push", "error", err)
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "pushed"})
	})

	srv := &http.Server{Handler: s.withServerHeader(s.logRequests(mux))}
	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()

	if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		s.logger.Error("webhook server stopped", "error", err)
	}
}

func (s *Server) authorizeWebhook(r *http.Request) bool {
	secret := s.cfg.WebhookAuthPreShared
	if secret == "" {
		return true
	}

	header := strings.TrimSpace(r.Header.Get("Authorization"))
	const prefix = "Bearer "
	if header == "" || !strings.HasPrefix(header, prefix) {
		return false
	}

	token := strings.TrimSpace(header[len(prefix):])
	if token == "" {
		return false
	}

	if subtle.ConstantTimeCompare([]byte(token), []byte(secret)) != 1 {
		return false
	}

	return true
}

func (s *Server) tryStatic(w http.ResponseWriter, r *http.Request) bool {
	clean := sanitizeRequestPath(r.URL.Path)
	if clean == "/" {
		return false
	}
	ext := strings.ToLower(filepath.Ext(clean))
	if ext == "" || ext == ".html" {
		return false
	}
	target := filepath.Join(s.cfg.OutputDir, filepath.FromSlash(strings.TrimPrefix(clean, "/")))
	if !isWithin(s.cfg.OutputDir, target) {
		return false
	}
	info, err := os.Stat(target)
	if err != nil || info.IsDir() {
		return false
	}
	http.ServeFile(w, r, target)
	return true
}

func isWithin(base, target string) bool {
	baseAbs, err := filepath.Abs(base)
	if err != nil {
		return false
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(baseAbs, targetAbs)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return false
	}
	return true
}

func sanitizeRequestPath(p string) string {
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	clean := path.Clean(p)
	if clean == "." {
		return "/"
	}
	return clean
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}
