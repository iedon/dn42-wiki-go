package site

import (
	"path"
	"path/filepath"
	"strings"
)

// StaticDocumentPath resolves the on-disk HTML file corresponding to a request path.
func (s *Service) StaticDocumentPath(requestPath string) (string, error) {
	route := sanitizeRoute(requestPath)
	if route == "/" {
		return filepath.Join(s.cfg.OutputDir, "index.html"), nil
	}
	if route == directoryPageRoute {
		return filepath.Join(s.cfg.OutputDir, directoryPageOutput), nil
	}

	trimmed := strings.TrimPrefix(route, "/")
	trimmed = strings.TrimSuffix(trimmed, "/")
	lower := strings.ToLower(trimmed)
	if strings.HasSuffix(lower, ".html") {
		trimmed = trimmed[:len(trimmed)-len(".html")]
	}

	mdPath, err := normalizeRelPath(trimmed, s.cfg.HomeDoc)
	if err != nil {
		return "", err
	}
	htmlPath := htmlPathFrom(mdPath)
	return filepath.Join(s.cfg.OutputDir, filepath.FromSlash(htmlPath)), nil
}

// NotFoundDocumentPath returns the static 404 page path.
func (s *Service) NotFoundDocumentPath() string {
	return filepath.Join(s.cfg.OutputDir, "404.html")
}

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
