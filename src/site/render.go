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
	if err := s.refreshLayout(ctx); err != nil {
		return nil, err
	}

	norm, err := normalizeRelPath(relPath)
	if err != nil {
		return nil, err
	}

	if isDirectoryRoute(norm) {
		return s.directoryPageData(), nil
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
	if err := s.refreshLayout(ctx); err != nil {
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
	data.RequestedPath = sanitizeRequestedPath(requestedPath)

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

	return &templatex.PageData{
		Title:            doc.Title,
		HeaderHTML:       snapshot.Header,
		FooterHTML:       snapshot.Footer,
		ServerFooterHTML: snapshot.ServerFooter,
		SidebarHTML:      snapshot.Sidebar,
		ContentHTML:      doc.HTML,
		ContentTemplate:  templatex.DefaultContentTemplate,
		Sections:         doc.Sections,
		ActivePath:       doc.Route,
		RequestedPath:    doc.Route,
		SearchEnabled:    true,
		Editable:         s.cfg.Editable,
		Buttons: templatex.PageButtons{
			EnableHistory: s.cfg.Editable,
			EnableRename:  s.cfg.Editable,
			EnableEdit:    s.cfg.Editable,
			EnableNew:     s.cfg.Editable,
		},
		SearchIndexURL:  path.Join("/", s.cfg.BaseURL, "search-index.json"),
		Live:            s.cfg.Live,
		RepositoryURL:   s.cfg.Git.Remote,
		BaseURL:         s.cfg.BaseURL,
		Breadcrumbs:     buildBreadcrumbs(doc.Route, doc.Title, s.cfg.BaseURL),
		LastUpdatedISO:  lastUpdatedISO,
		LastUpdated:     lastUpdated,
		LastCommitHash:  doc.LastHash,
		LastCommitShort: lastCommitShort,
	}
}

func (s *Service) directoryPageData() *templatex.PageData {
	snapshot := s.layout.Snapshot()
	content := snapshot.Sidebar
	if len(content) == 0 {
		content = template.HTML("<p>No sidebar content available.</p>")
	}

	return &templatex.PageData{
		Title:            directoryPageTitle,
		HeaderHTML:       snapshot.Header,
		FooterHTML:       snapshot.Footer,
		ServerFooterHTML: snapshot.ServerFooter,
		SidebarHTML:      template.HTML(""),
		ContentHTML:      content,
		ContentTemplate:  templatex.DefaultContentTemplate,
		ActivePath:       directoryPageRoute,
		RequestedPath:    directoryPageRoute,
		SearchEnabled:    true,
		Editable:         s.cfg.Editable,
		Buttons:          templatex.PageButtons{},
		SearchIndexURL:   path.Join("/", s.cfg.BaseURL, "search-index.json"),
		Live:             s.cfg.Live,
		RepositoryURL:    s.cfg.Git.Remote,
		BaseURL:          s.cfg.BaseURL,
		Breadcrumbs: []templatex.Breadcrumb{
			{Title: directoryPageTitle, Current: true},
		},
	}
}

func (s *Service) writeDocuments(docs []page) error {
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

		target := filepath.Join(s.cfg.OutputDir, filepath.FromSlash(doc.OutputPath))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, minified, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) writeDirectoryPage() error {
	data := s.directoryPageData()
	var buf bytes.Buffer
	if err := s.templates.Render(&buf, data); err != nil {
		return err
	}
	minified, err := s.renderer.MinifyHTML(buf.Bytes())
	if err != nil {
		return fmt.Errorf("minify directory page: %w", err)
	}
	output := filepath.Join(s.cfg.OutputDir, directoryPageOutput)
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	return os.WriteFile(output, minified, 0o644)
}

func (s *Service) writeHomeAliases(docs []page) error {
	for _, doc := range docs {
		if strings.EqualFold(doc.Source, "Home.md") {
			alias := filepath.Join(s.cfg.OutputDir, "index.html")
			target := filepath.Join(s.cfg.OutputDir, filepath.FromSlash(doc.OutputPath))
			if err := fsutil.CopyFile(target, alias); err != nil {
				return fmt.Errorf("create home alias: %w", err)
			}
			break
		}
	}
	return nil
}
