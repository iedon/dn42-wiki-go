package site

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"

	"github.com/iedon/dn42-wiki-go/gitutil"
	"github.com/iedon/dn42-wiki-go/renderer"
	"github.com/iedon/dn42-wiki-go/templatex"
)

// DocumentStore wraps Git repository access and Markdown rendering.
type DocumentStore struct {
	repo     *gitutil.Repository
	renderer *renderer.Renderer
	homeDoc  string
}

func newDocumentStore(repo *gitutil.Repository, renderer *renderer.Renderer, homeDoc string) *DocumentStore {
	return &DocumentStore{repo: repo, renderer: renderer, homeDoc: ensureHomeDoc(homeDoc)}
}

func (d *DocumentStore) ListTracked(ctx context.Context) ([]string, error) {
	files, err := d.repo.ListTrackedFiles(ctx)
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func (d *DocumentStore) Read(relPath string) ([]byte, error) {
	return d.repo.ReadFile(relPath)
}

func (d *DocumentStore) Write(relPath string, content []byte) error {
	return d.repo.WriteFile(relPath, content)
}

func (d *DocumentStore) RenderDocument(ctx context.Context, relPath string) (page, error) {
	data, err := d.repo.ReadFile(relPath)
	if err != nil {
		return page{}, fmt.Errorf("read %s: %w", relPath, err)
	}

	rendered, err := d.renderer.Render(data)
	if err != nil {
		return page{}, fmt.Errorf("render %s: %w", relPath, err)
	}

	sections := make([]templatex.TOCEntry, 0, len(rendered.Headings))
	for _, heading := range rendered.Headings {
		sections = append(sections, templatex.TOCEntry{ID: heading.ID, Text: heading.Text, Level: heading.Level})
	}

	title := deriveTitle(relPath)
	summary := summarize(rendered.PlainText)

	doc := page{
		Source:     relPath,
		Route:      routeFromPath(relPath, d.homeDoc),
		OutputPath: htmlPathFrom(relPath, d.homeDoc),
		Title:      title,
		HTML:       template.HTML(rendered.HTML),
		Sections:   sections,
		Summary:    summary,
		PlainText:  rendered.PlainText,
	}
	if commits, _, err := d.repo.Log(ctx, relPath, 0, 1); err == nil && len(commits) > 0 {
		doc.LastHash = commits[0].Hash
		doc.LastMod = commits[0].CommittedAt
	}
	return doc, nil
}

func (d *DocumentStore) RenderFragment(name string) (*renderer.RenderResult, error) {
	bytes, err := d.repo.ReadFile(name)
	if err != nil {
		return nil, err
	}
	return d.renderer.Render(bytes)
}

func (d *DocumentStore) Rename(ctx context.Context, oldPath, newPath string) error {
	return d.repo.Rename(ctx, oldPath, newPath)
}

func (d *DocumentStore) Commit(ctx context.Context, paths []string, message, author string) error {
	return d.repo.CommitChanges(ctx, paths, message, author)
}

func (d *DocumentStore) Diff(ctx context.Context, relPath, from, to string) (string, error) {
	return d.repo.Diff(ctx, relPath, from, to)
}

func (d *DocumentStore) History(ctx context.Context, relPath string, page, pageSize int) ([]gitutil.Commit, bool, error) {
	return d.repo.Log(ctx, relPath, page, pageSize)
}

func (d *DocumentStore) RepoDir() string {
	return d.repo.Dir
}

func (d *DocumentStore) Exists(rel string) (bool, error) {
	full := filepath.Join(d.repo.Dir, filepath.FromSlash(rel))
	info, err := os.Stat(full)
	if err == nil {
		return !info.IsDir(), nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}
