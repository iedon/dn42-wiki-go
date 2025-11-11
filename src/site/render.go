package site

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/iedon/dn42-wiki-go/fsutil"
	"github.com/iedon/dn42-wiki-go/templatex"
)

// RenderPage renders a single document for live mode.
func (s *Service) RenderPage(ctx context.Context, relPath string) (*templatex.PageData, error) {
	if err := s.buildLayout(ctx); err != nil {
		return nil, err
	}

	norm, err := normalizeRelPath(relPath, s.cfg.HomeDoc)
	if err != nil {
		return nil, err
	}
	if err := s.ensureRouteAccessible(norm); err != nil {
		return nil, err
	}

	if isDirectoryRoute(norm) {
		return s.directoryPageData(ctx)
	}

	doc, err := s.documents.RenderDocument(ctx, norm)
	if err != nil {
		return nil, err
	}
	return s.pageData(doc), nil
}

// RenderFullPage renders and minifies a page ready to be written to the response.
func (s *Service) RenderFullPage(ctx context.Context, relPath string) ([]byte, error) {
	data, err := s.RenderPage(ctx, relPath)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := s.templates.Render(&buf, data); err != nil {
		return nil, err
	}
	return s.renderer.MinifyHTML(buf.Bytes())
}

// RenderNotFoundPage renders a themed 404 page.
func (s *Service) RenderNotFoundPage(ctx context.Context, requestedPath string) ([]byte, error) {
	if err := s.buildLayout(ctx); err != nil {
		return nil, err
	}

	doc := page{
		Title: "404 - Not found",
		Route: "",
		HTML:  template.HTML(""),
	}

	data := s.pageData(doc)
	data.Editable = false
	data.Buttons = templatex.PageButtons{}
	data.ContentTemplate = templatex.NotFoundContentTemplate
	data.ActivePath = ""
	sanitized := strings.TrimSpace(requestedPath)
	if sanitized != "" {
		sanitized = sanitizeRequestedPath(sanitized)
	}
	data.RequestedPath = sanitized
	description := "The page you are looking for could not be found."
	if sanitized != "" && sanitized != "/" {
		description = fmt.Sprintf("The requested path %s could not be found.", sanitized)
	}
	data.Meta = s.buildMeta(description, description, "website")

	var buf bytes.Buffer
	if err := s.templates.Render(&buf, data); err != nil {
		return nil, err
	}
	return s.renderer.MinifyHTML(buf.Bytes())
}

func sanitizeRequestedPath(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "/")
	cleaned := path.Clean("/" + trimmed)
	if cleaned == "." || cleaned == "" {
		return "/"
	}
	if !strings.HasPrefix(cleaned, "/") {
		cleaned = "/" + cleaned
	}
	return cleaned
}

// RenderForbiddenPage renders a themed 403 page for restricted routes.
func (s *Service) RenderForbiddenPage(ctx context.Context, requestedPath string) ([]byte, error) {
	if err := s.buildLayout(ctx); err != nil {
		return nil, err
	}

	doc := page{
		Title: "403 - Forbidden",
		Route: "",
		HTML:  template.HTML(""),
	}

	data := s.pageData(doc)
	data.Editable = false
	data.Buttons = templatex.PageButtons{}
	data.ContentTemplate = templatex.ForbiddenContentTemplate
	data.ActivePath = ""
	sanitized := strings.TrimSpace(requestedPath)
	if sanitized != "" {
		sanitized = sanitizeRequestedPath(sanitized)
	}
	data.RequestedPath = sanitized
	description := "Access to the requested resource is restricted."
	if sanitized != "" && sanitized != "/" {
		description = fmt.Sprintf("Access to %s is restricted.", sanitized)
	}
	data.Meta = s.buildMeta(description, doc.Title, "website")

	var buf bytes.Buffer
	if err := s.templates.Render(&buf, data); err != nil {
		return nil, err
	}
	return s.renderer.MinifyHTML(buf.Bytes())
}

func (s *Service) renderDocuments(ctx context.Context, files []string) ([]page, error) {
	docs := make([]page, 0, len(files))
	for _, file := range files {
		if !isMarkdown(file) || isLayoutFragment(file) {
			continue
		}
		doc, err := s.documents.RenderDocument(ctx, file)
		if err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	sort.Slice(docs, func(i, j int) bool {
		return docs[i].Route < docs[j].Route
	})
	return docs, nil
}

func (s *Service) pageData(doc page) *templatex.PageData {
	snapshot := s.layout.Snapshot()
	pageTitle := s.pageTitle(doc.Title)

	var lastUpdatedISO, lastUpdated, lastCommitShort string
	if !doc.LastMod.IsZero() {
		lastUpdatedISO = doc.LastMod.UTC().Format(time.RFC3339)
		lastUpdated = doc.LastMod.UTC().Format("Jan 2 15:04:05 MST 2006")
	}
	if doc.LastHash != "" {
		if len(doc.LastHash) > 12 {
			lastCommitShort = doc.LastHash[:12]
		} else {
			lastCommitShort = doc.LastHash
		}
	}

	data := &templatex.PageData{
		Title:            doc.Title,
		PageTitle:        pageTitle,
		HeaderHTML:       snapshot.Header,
		FooterHTML:       snapshot.Footer,
		ServerFooterHTML: snapshot.ServerFooter,
		SidebarHTML:      snapshot.Sidebar,
		ContentHTML:      doc.HTML,
		ContentTemplate:  templatex.DefaultContentTemplate,
		Sections:         doc.Sections,
		ActivePath:       doc.Route,
		RequestedPath:    doc.Route,
		Editable:         s.cfg.Editable,
		Buttons: templatex.PageButtons{
			EnableHistory: s.cfg.Editable,
			EnableRename:  s.cfg.Editable,
			EnableEdit:    s.cfg.Editable,
			EnableNew:     s.cfg.Editable,
		},
		SearchIndexURL:  path.Join("/", s.cfg.BaseURL, "search-index.json"),
		Live:            s.cfg.Live,
		BaseURL:         s.cfg.BaseURL,
		Breadcrumbs:     buildBreadcrumbs(doc.Route, doc.Title, s.cfg.BaseURL),
		LastUpdatedISO:  lastUpdatedISO,
		LastUpdated:     lastUpdated,
		LastCommitHash:  doc.LastHash,
		LastCommitShort: lastCommitShort,
	}
	data.Meta = s.buildMeta(doc.Summary, doc.Title, "article")
	return data
}

func (s *Service) directoryPageData(ctx context.Context) (*templatex.PageData, error) {
	snapshot := s.layout.Snapshot()
	entries, err := s.directoryEntries(ctx)
	if err != nil {
		return nil, err
	}
	title := directoryPageTitle

	data := &templatex.PageData{
		Title:            title,
		PageTitle:        s.pageTitle(title),
		HeaderHTML:       snapshot.Header,
		FooterHTML:       snapshot.Footer,
		ServerFooterHTML: snapshot.ServerFooter,
		SidebarHTML:      snapshot.Sidebar,
		ContentTemplate:  templatex.DirectoryContentTemplate,
		ActivePath:       directoryPageRoute,
		RequestedPath:    directoryPageRoute,
		Editable:         false,
		Buttons:          templatex.PageButtons{},
		SearchIndexURL:   path.Join("/", s.cfg.BaseURL, "search-index.json"),
		Live:             s.cfg.Live,
		BaseURL:          s.cfg.BaseURL,
		Breadcrumbs: []templatex.Breadcrumb{
			{Title: directoryPageTitle, Current: true},
		},
		Directory: entries,
	}
	data.Meta = s.buildMeta("Browse the complete documentation index.", directoryPageTitle, "website")
	return data, nil
}

func (s *Service) writeDocuments(baseDir string, docs []page) error {
	for _, doc := range docs {
		data := s.pageData(doc)
		var buf bytes.Buffer
		if err := s.templates.Render(&buf, data); err != nil {
			return err
		}

		minified, err := s.renderer.MinifyHTML(buf.Bytes())
		if err != nil {
			return fmt.Errorf("minify %s: %w", doc.Route, err)
		}

		target := filepath.Join(baseDir, filepath.FromSlash(doc.OutputPath))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, minified, 0o644); err != nil {
			return err
		}
		if !doc.LastMod.IsZero() {
			stamp := doc.LastMod.UTC()
			if err := os.Chtimes(target, stamp, stamp); err != nil {
				return fmt.Errorf("set mod time %s: %w", doc.Route, err)
			}
		}
	}
	return nil
}

func (s *Service) writeDirectoryPage(ctx context.Context, baseDir string) error {
	data, err := s.directoryPageData(ctx)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := s.templates.Render(&buf, data); err != nil {
		return err
	}
	minified, err := s.renderer.MinifyHTML(buf.Bytes())
	if err != nil {
		return fmt.Errorf("minify directory page: %w", err)
	}
	output := filepath.Join(baseDir, directoryPageOutput)
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	return os.WriteFile(output, minified, 0o644)
}

func (s *Service) writeNotFoundPage(ctx context.Context, baseDir string) error {
	// Pre-render a themed 404 page so static hosting setups can serve it directly.
	pageBytes, err := s.RenderNotFoundPage(ctx, "")
	if err != nil {
		return err
	}
	output := filepath.Join(baseDir, "404.html")
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	return os.WriteFile(output, pageBytes, 0o644)
}

func (s *Service) writeForbiddenPage(ctx context.Context, baseDir string) error {
	pageBytes, err := s.RenderForbiddenPage(ctx, "")
	if err != nil {
		return err
	}
	output := filepath.Join(baseDir, "403.html")
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	return os.WriteFile(output, pageBytes, 0o644)
}

func (s *Service) writeHomeAliases(baseDir string, docs []page) error {
	for _, doc := range docs {
		if strings.EqualFold(doc.Source, ensureHomeDoc(s.cfg.HomeDoc)) {
			alias := filepath.Join(baseDir, "index.html")
			target := filepath.Join(baseDir, filepath.FromSlash(doc.OutputPath))
			if err := fsutil.CopyFile(target, alias); err != nil {
				return fmt.Errorf("create home alias: %w", err)
			}
			if !doc.LastMod.IsZero() {
				stamp := doc.LastMod.UTC()
				if err := os.Chtimes(alias, stamp, stamp); err != nil {
					return fmt.Errorf("alias mod time: %w", err)
				}
			}
			break
		}
	}
	return nil
}

func (s *Service) buildMeta(summary, fallback, ogType string) templatex.Meta {
	if ogType == "" {
		ogType = "website"
	}
	description := metaDescription(summary, fallback)
	if description == "" {
		description = s.siteName()
	}
	return templatex.Meta{
		Description:   description,
		OpenGraphType: ogType,
		OpenGraphSite: s.siteName(),
	}
}

func (s *Service) siteName() string {
	name := strings.TrimSpace(s.cfg.SiteName)
	if name == "" {
		name = deriveTitle(ensureHomeDoc(s.cfg.HomeDoc))
	}
	name = strings.TrimSpace(name)
	if name == "" || strings.EqualFold(name, "Untitled") {
		return "Untitled"
	}
	return name
}

func (s *Service) pageTitle(raw string) string {
	title := strings.TrimSpace(raw)
	site := s.siteName()
	if title == "" {
		return site
	}
	if site == "" {
		return title
	}
	return fmt.Sprintf("%s - %s", title, site)
}
