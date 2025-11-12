package site

import (
	"path"
	"path/filepath"
	"strings"
)

// StaticDocumentPath resolves the on-disk HTML file corresponding to a request path.
func (s *Service) StaticDocumentPath(requestPath string) (string, error) {
	info, ok := s.analyzeRequestPath(requestPath)
	if !ok {
		return "", ErrInvalidPath
	}
	switch info.relative {
	case "/":
		return filepath.Join(s.cfg.OutputDir, "index.html"), nil
	case directoryPageRoute:
		return filepath.Join(s.cfg.OutputDir, directoryPageOutput), nil
	}

	_, _, htmlPath, err := info.documentTargets(s.homeDoc)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.cfg.OutputDir, filepath.FromSlash(htmlPath)), nil
}

// NotFoundDocumentPath returns the static 404 page path.
func (s *Service) NotFoundDocumentPath() string {
	return filepath.Join(s.cfg.OutputDir, "404.html")
}

// ForbiddenDocumentPath returns the static 403 page path.
func (s *Service) ForbiddenDocumentPath() string {
	return filepath.Join(s.cfg.OutputDir, "403.html")
}

// sanitizeRoute cleans and normalizes an HTTP route.
func sanitizeRoute(input string) string {
	route := strings.TrimSpace(input)
	if route == "" {
		return "/"
	}
	if !strings.HasPrefix(route, "/") {
		route = "/" + route
	}
	cleaned := path.Clean(route)
	if cleaned == "." {
		cleaned = "/"
	}
	return cleaned
}
