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

func metaDescription(summary, fallback string) string {
	const limit = 160
	text := strings.TrimSpace(summary)
	if text == "" {
		text = strings.TrimSpace(fallback)
	}
	if text == "" {
		return ""
	}
	text = strings.Join(strings.Fields(text), " ")
	text = fixPunctuationSpacing(text)
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	cutoff := min(limit, len(runes))
	trimmed := strings.TrimSpace(string(runes[:cutoff]))
	if idx := strings.LastIndex(trimmed, " "); idx > -1 {
		trimmed = trimmed[:idx]
	}
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" {
		return ""
	}
	return trimmed + "..."
}

func fixPunctuationSpacing(s string) string {
	replacer := strings.NewReplacer(
		" ,", ",",
		" .", ".",
		" ;", ";",
		" :", ":",
		" !", "!",
		" ?", "?",
		" %", "%",
		" )", ")",
		" ]", "]",
		" }", "}",
		" '", "'",
		" \"", "\"",
		"( ", "(",
		"[ ", "[",
		"{ ", "{",
	)
	return replacer.Replace(s)
}
