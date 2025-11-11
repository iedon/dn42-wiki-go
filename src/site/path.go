package site

import (
	"errors"
	"path"
	"path/filepath"
	"strings"
)

var reservedRouteNames = map[string]struct{}{
	"index":        {},
	"_sidebar":     {},
	"_footer":      {},
	"404":          {},
	"403":          {},
	"_header":      {},
	"layout":       {},
	"readme":       {},
	"search-index": {},
	"directory":    {},
	"gollum":       {},
	"root":         {},
	"default":      {},
	"assets":       {},
	"api":          {},
}

var (
	// ErrInvalidPath is returned when user-provided routes fail validation.
	ErrInvalidPath = errors.New("invalid path")
	// ErrReservedPath indicates the caller attempted to use a reserved route name.
	ErrReservedPath = errors.New("reserved path")
	// ErrForbiddenRoute indicates the requested route is configured as private.
	ErrForbiddenRoute = errors.New("route is restricted")
)

func normalizeRelPath(input, homeDoc string) (string, error) {
	home := ensureHomeDoc(homeDoc)
	candidate := strings.TrimSpace(input)
	candidate = strings.ReplaceAll(candidate, "\\", "/")
	candidate = strings.Trim(candidate, "/")
	if candidate == "" {
		candidate = home
	}
	if strings.Contains(candidate, "\x00") {
		return "", errors.Join(ErrInvalidPath, errors.New("contains null byte"))
	}
	if !strings.HasSuffix(strings.ToLower(candidate), ".md") {
		candidate += ".md"
	}

	cleaned := path.Clean(candidate)
	for strings.HasPrefix(cleaned, "./") {
		cleaned = strings.TrimPrefix(cleaned, "./")
	}
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "" {
		cleaned = home
	}
	if strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, "/../") {
		return "", errors.Join(ErrInvalidPath, errors.New("path escapes repository root"))
	}

	segments := strings.Split(cleaned, "/")
	for _, segment := range segments {
		if segment == "" || segment == "." || segment == ".." {
			return "", errors.Join(ErrInvalidPath, errors.New("invalid path segment"))
		}
		if strings.HasPrefix(segment, "-") {
			return "", errors.Join(ErrInvalidPath, errors.New("path segment cannot start with '-'"))
		}
		if strings.Contains(segment, "\x00") {
			return "", errors.Join(ErrInvalidPath, errors.New("invalid path segment"))
		}
	}

	return filepath.ToSlash(cleaned), nil
}

// ensureHomeDoc normalizes the home document path.
func ensureHomeDoc(homeDoc string) string {
	trimmed := strings.TrimSpace(homeDoc)
	trimmed = strings.ReplaceAll(trimmed, "\\", "/")
	if trimmed == "" {
		trimmed = "Home.md"
	}
	if !strings.HasSuffix(strings.ToLower(trimmed), ".md") {
		trimmed += ".md"
	}
	cleaned := path.Clean(trimmed)
	for strings.HasPrefix(cleaned, "./") {
		cleaned = strings.TrimPrefix(cleaned, "./")
	}
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "" {
		cleaned = "Home.md"
	}
	return filepath.ToSlash(cleaned)
}

// isReservedPath checks if the given relative path maps to a reserved route.
func isReservedPath(rel string) bool {
	lowered := strings.ToLower(filepath.ToSlash(strings.TrimSpace(rel)))
	lowered = strings.TrimPrefix(lowered, "/")
	lowered = strings.TrimSuffix(lowered, ".md")
	if strings.Contains(lowered, "/") {
		return false
	}
	_, ok := reservedRouteNames[lowered]
	return ok
}

// isDirectoryRoute checks if the given relative path maps to the directory page route.
func isDirectoryRoute(rel string) bool {
	lowered := strings.ToLower(filepath.ToSlash(strings.TrimSpace(rel)))
	lowered = strings.TrimPrefix(lowered, "/")
	lowered = strings.TrimSuffix(lowered, ".md")
	lowered = strings.TrimSuffix(lowered, "/")
	return lowered == strings.TrimPrefix(strings.ToLower(directoryPageRoute), "/")
}

func isMarkdown(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".md")
}

func isLayoutFragment(path string) bool {
	base := filepath.Base(path)
	return base == "_Header.md" || base == "_Footer.md" || base == "_Sidebar.md"
}

func isIgnorable(path string) bool {
	base := filepath.Base(path)
	return strings.HasPrefix(base, ".git")
}

func routeFromPath(relPath, homeDoc string) string {
	normalized := filepath.ToSlash(strings.TrimSpace(relPath))
	home := filepath.ToSlash(strings.TrimSpace(homeDoc))
	if home == "" {
		home = "Home.md"
	}
	if strings.EqualFold(normalized, home) {
		return "/"
	}
	base := strings.TrimSuffix(normalized, filepath.Ext(normalized))
	base = strings.Trim(base, "/")
	if base == "" {
		return "/"
	}
	return "/" + base + "/"
}

func htmlPathFrom(relPath, homeDoc string) string {
	normalized := filepath.ToSlash(strings.TrimSpace(relPath))
	home := filepath.ToSlash(strings.TrimSpace(homeDoc))
	if home == "" {
		home = "Home.md"
	}
	if strings.EqualFold(normalized, home) {
		return "index.html"
	}
	base := strings.TrimSuffix(normalized, filepath.Ext(normalized))
	base = strings.Trim(base, "/")
	if base == "" {
		return "index.html"
	}
	return path.Join(base, "index.html")
}
