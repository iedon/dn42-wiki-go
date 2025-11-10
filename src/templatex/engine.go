package templatex

import (
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	DefaultContentTemplate   = "content-default"
	NotFoundContentTemplate  = "content-404"
	DirectoryContentTemplate = "content-directory"
	LayoutTemplate           = "layout"
)

// Engine is a thin wrapper around Go templates with a fallback default layout.
type Engine struct {
	templates *template.Template
	StaticDir string
}

// PageData represents the data model expected by the default layout.
type PageData struct {
	Title            string
	PageTitle        string
	HeaderHTML       template.HTML
	FooterHTML       template.HTML
	ServerFooterHTML template.HTML
	SidebarHTML      template.HTML
	ContentHTML      template.HTML
	ContentTemplate  string
	Sections         []TOCEntry
	ActivePath       string
	RequestedPath    string
	SearchEnabled    bool
	Editable         bool
	Buttons          PageButtons
	SearchIndexURL   string
	Live             bool
	BaseURL          string
	Breadcrumbs      []Breadcrumb
	LastUpdatedISO   string
	LastUpdated      string
	LastCommitHash   string
	LastCommitShort  string
	Directory        []*DirectoryEntry
	Meta             Meta
}

// Meta holds SEO-oriented metadata for the rendered page.
type Meta struct {
	Description   string
	OpenGraphType string
	OpenGraphSite string
}

// TOCEntry models a single heading for sidebar navigation.
type TOCEntry struct {
	ID    string
	Text  string
	Level int
}

// PageButtons controls the visibility of editing actions.
type PageButtons struct {
	EnableHistory bool
	EnableRename  bool
	EnableEdit    bool
	EnableNew     bool
}

// Breadcrumb models a single breadcrumb entry for navigation.
type Breadcrumb struct {
	Title   string
	Path    string
	Current bool
}

// DirectoryEntry represents a node in the directory listing hierarchy.
type DirectoryEntry struct {
	Title    string
	URL      string
	Route    string
	Children []*DirectoryEntry
	Count    int
	Depth    int
	ID       string
	Anchor   string
	Aliases  []string
}

// Load instantiates an engine using files from templateDir.
func Load(templateDir string) (*Engine, error) {
	if templateDir == "" {
		return nil, fmt.Errorf("template directory not configured")
	}

	engine := &Engine{}

	funcs := template.FuncMap{
		"safeHTML": func(v any) template.HTML {
			switch value := v.(type) {
			case template.HTML:
				return value
			case string:
				return template.HTML(value)
			default:
				return ""
			}
		},
		"baseHref": func(base string) string {
			base = strings.TrimSpace(base)
			if base == "" || base == "/" {
				return "/"
			}
			trimmed := strings.Trim(base, "/")
			return "/" + trimmed + "/"
		},
	}

	files := make([]string, 0)
	mainPattern := filepath.Join(templateDir, "*.html")
	mainFiles, err := filepath.Glob(mainPattern)
	if err != nil {
		return nil, fmt.Errorf("glob main templates: %w", err)
	}
	files = append(files, mainFiles...)

	partialsDir := filepath.Join(templateDir, "partials")
	if info, err := os.Stat(partialsDir); err == nil && info.IsDir() {
		partialPattern := filepath.Join(partialsDir, "*.html")
		partialFiles, err := filepath.Glob(partialPattern)
		if err != nil {
			return nil, fmt.Errorf("glob partial templates: %w", err)
		}
		files = append(files, partialFiles...)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no templates found in %s", templateDir)
	}

	sort.Strings(files)

	tpl, err := template.New("root").Funcs(funcs).ParseFiles(files...)
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	if tpl.Lookup(LayoutTemplate) == nil {
		return nil, fmt.Errorf("template %q is not defined", LayoutTemplate)
	}

	engine.templates = tpl

	assetsPath := filepath.Join(templateDir, "assets")
	if info, err := os.Stat(assetsPath); err == nil && info.IsDir() {
		engine.StaticDir = assetsPath
	}

	return engine, nil
}

// Render writes the rendered layout into the provided writer.
func (e *Engine) Render(w io.Writer, data *PageData) error {
	if e.templates == nil {
		return fmt.Errorf("template engine not initialized")
	}
	if data != nil {
		if strings.TrimSpace(data.ContentTemplate) == "" {
			data.ContentTemplate = DefaultContentTemplate
		}
		if strings.TrimSpace(data.RequestedPath) == "" {
			data.RequestedPath = data.ActivePath
		}
	}
	return e.templates.ExecuteTemplate(w, LayoutTemplate, data)
}
