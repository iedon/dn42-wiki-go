package site

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/iedon/dn42-wiki-go/config"
	"github.com/iedon/dn42-wiki-go/fsutil"
	"github.com/iedon/dn42-wiki-go/gitutil"
	"github.com/iedon/dn42-wiki-go/renderer"
	"github.com/iedon/dn42-wiki-go/templatex"
)

// Service orchestrates document rendering, indexing, and persistence.
type Service struct {
	cfg       *config.Config
	repo      *gitutil.Repository
	templates *templatex.Engine
	renderer  *renderer.Renderer

	documents *DocumentStore
	layout    *LayoutCache
	search    *SearchCatalog
}

// NewService constructs a Service instance.
func NewService(cfg *config.Config, repo *gitutil.Repository, templates *templatex.Engine) *Service {
	rend := renderer.New()
	return &Service{
		cfg:       cfg,
		repo:      repo,
		templates: templates,
		renderer:  rend,
		documents: newDocumentStore(repo, rend),
		layout:    newLayoutCache(),
		search:    newSearchCatalog(),
	}
}

// Warm loads layout fragments and rebuilds the in-memory search data.
func (s *Service) Warm(ctx context.Context) error {
	if err := s.refreshLayout(ctx); err != nil {
		return err
	}
	return s.rebuildSearchIndex(ctx)
}

// BuildStatic renders the entire repository into static HTML assets.
func (s *Service) BuildStatic(ctx context.Context) error {
	finalDir := s.cfg.OutputDir
	parent := filepath.Dir(finalDir)
	if parent == "" {
		parent = "."
	}

	tempDir, err := os.MkdirTemp(parent, ".__build-")
	if err != nil {
		return fmt.Errorf("create temp output dir: %w", err)
	}
	cleanTemp := true
	defer func() {
		if cleanTemp && tempDir != "" {
			_ = os.RemoveAll(tempDir)
		}
	}()

	if err := s.refreshLayout(ctx); err != nil {
		return err
	}

	files, err := s.documents.ListTracked(ctx)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("repository has no tracked files")
	}

	docs, err := s.renderDocuments(ctx, files)
	if err != nil {
		return err
	}

	for _, file := range files {
		if isMarkdown(file) || isIgnorable(file) || isLayoutFragment(file) {
			continue
		}
		src := filepath.Join(s.repo.Dir, filepath.FromSlash(file))
		dst := filepath.Join(tempDir, filepath.FromSlash(file))
		if err := fsutil.CopyFile(src, dst); err != nil {
			return fmt.Errorf("copy asset %s: %w", file, err)
		}
	}

	if err := s.writeDocuments(tempDir, docs); err != nil {
		return err
	}
	if err := s.writeDirectoryPage(ctx, tempDir); err != nil {
		return err
	}
	if err := s.writeNotFoundPage(ctx, tempDir); err != nil {
		return err
	}

	indexJSON, err := buildSearchIndex(docs)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(tempDir, "search-index.json"), indexJSON, 0o644); err != nil {
		return fmt.Errorf("write search index: %w", err)
	}
	s.search.Update(indexJSON)

	if s.templates.StaticDir != "" {
		dst := filepath.Join(tempDir, "theme")
		if err := fsutil.CopyTree(s.templates.StaticDir, dst); err != nil {
			return fmt.Errorf("copy theme assets: %w", err)
		}
	}

	if err := s.writeHomeAliases(tempDir, docs); err != nil {
		return err
	}

	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("ensure output parent: %w", err)
	}

	backupDir := finalDir + ".old"
	if err := os.RemoveAll(backupDir); err != nil {
		return fmt.Errorf("clean backup dir: %w", err)
	}

	if err := os.Rename(finalDir, backupDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("rotate old output: %w", err)
	}

	if err := os.Rename(tempDir, finalDir); err != nil {
		_ = os.Rename(backupDir, finalDir)
		return fmt.Errorf("activate new output: %w", err)
	}

	_ = os.RemoveAll(backupDir)
	cleanTemp = false
	tempDir = ""
	return nil
}

// RenderPreview renders markdown content without persisting it.
func (s *Service) RenderPreview(content []byte) (*renderer.RenderResult, error) {
	return s.renderer.Render(content)
}

// SearchIndex returns a snapshot of the current search dataset.
func (s *Service) SearchIndex() json.RawMessage {
	payload := s.search.Snapshot()
	if len(payload) == 0 {
		return append(json.RawMessage(nil), emptySearchIndexJSON...)
	}
	return payload
}

// Pull synchronizes the repository and refreshes caches.
func (s *Service) Pull(ctx context.Context) error {
	if err := s.repo.Pull(ctx); err != nil {
		return err
	}
	if err := s.Warm(ctx); err != nil {
		return err
	}
	if err := s.BuildStatic(ctx); err != nil {
		return fmt.Errorf("build static: %w", err)
	}
	return nil
}

// Push synchronizes local commits to the configured remote.
func (s *Service) Push(ctx context.Context) error {
	return s.repo.Push(ctx)
}

// RepositoryDir returns the path of the checked-out wiki repository.
func (s *Service) RepositoryDir() string {
	return s.documents.RepoDir()
}

// ThemeDir returns the directory containing template assets, if any.
func (s *Service) ThemeDir() string {
	return s.templates.StaticDir
}
