package site

import (
	"path/filepath"
	"strings"
)

func deriveTitle(relPath string) string {
	name := strings.TrimSuffix(filepath.Base(relPath), filepath.Ext(relPath))
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.TrimSpace(name)
	if name == "" {
		return "Untitled"
	}
	return name
}

func summarize(plain string) string {
	plain = strings.TrimSpace(plain)
	if plain == "" {
		return ""
	}
	if len(plain) <= 200 {
		return plain
	}
	return plain[:200] + "..."
}
