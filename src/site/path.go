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
	"_header":      {},
	"layout":       {},
	"readme":       {},
	"search-index": {},
	"directory":    {},
	"gollum":       {},
	"root":         {},
	"default":      {},
	"theme":        {},
}

var (
	// ErrInvalidPath is returned when user-provided routes fail validation.
	ErrInvalidPath = errors.New("invalid path")
	// ErrReservedPath indicates the caller attempted to use a reserved route name.
	ErrReservedPath = errors.New("reserved path")
)

func normalizeRelPath(input string) (string, error) {
	candidate := strings.TrimSpace(input)
	candidate = strings.ReplaceAll(candidate, "\\", "/")
	candidate = strings.Trim(candidate, "/")
	if candidate == "" {
		candidate = "Home"
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
		cleaned = "Home.md"
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

func routeFromPath(relPath string) string {
	slash := filepath.ToSlash(relPath)
	slash = strings.TrimSuffix(slash, filepath.Ext(slash))
	if !strings.HasPrefix(slash, "/") {
		slash = "/" + slash
	}
	return slash
}

func htmlPathFrom(relPath string) string {
	rel := filepath.ToSlash(relPath)
	rel = strings.TrimSuffix(rel, filepath.Ext(rel)) + ".html"
	return rel
}
