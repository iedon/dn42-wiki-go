package site

import (
	"html/template"
	"time"

	"github.com/iedon/dn42-wiki-go/templatex"
)

type page struct {
	Source     string
	Route      string
	OutputPath string
	Title      string
	HTML       template.HTML
	Sections   []templatex.TOCEntry
	Summary    string
	PlainText  string
	LastHash   string
	LastMod    time.Time
}
