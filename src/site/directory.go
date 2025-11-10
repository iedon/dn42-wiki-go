package site

import "strings"

const (
	directoryPageRoute  = "/directory"
	directoryPageOutput = "directory.html"
	directoryPageTitle  = "Directory"
)

func directoryPageHref(base string) string {
	if base == "" {
		return directoryPageRoute
	}
	return "/" + base + directoryPageRoute
}

func breadcrumbAnchor(segment string) string {
	segment = strings.TrimSpace(segment)
	if segment == "" {
		return ""
	}
	segment = strings.ToLower(segment)
	var b strings.Builder
	lastDash := false
	for _, r := range segment {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == ' ' || r == '-' || r == '_' || r == '.':
			if lastDash || b.Len() == 0 {
				continue
			}
			b.WriteByte('-')
			lastDash = true
		}
	}
	anchor := strings.Trim(b.String(), "-")
	return anchor
}
