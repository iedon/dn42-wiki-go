package site

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/iedon/dn42-wiki-go/gitutil"
)

// SavePage writes content to disk, stages, and commits the change.
func (s *Service) SavePage(ctx context.Context, relPath string, content []byte, message, remoteAddr string) error {
	if !s.cfg.Editable {
		return fmt.Errorf("editing disabled")
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	if err := s.ensureRepositoryFresh(ctx); err != nil {
		return err
	}

	rel, err := normalizeRelPath(relPath, s.homeDoc)
	if err != nil {
		return err
	}
	if err := s.ensureRouteAccessible(rel); err != nil {
		return err
	}
	exists, err := s.documents.Exists(rel)
	if err != nil {
		return err
	}
	if !exists && isReservedPath(rel) {
		return fmt.Errorf("%w: %s", ErrReservedPath, rel)
	}
	if err := s.documents.Write(rel, content); err != nil {
		return err
	}
	finalMessage, err := s.composeCommitMessage(message, remoteAddr)
	if err != nil {
		return err
	}
	finalAuthor := s.composeCommitAuthor("")
	if err := s.documents.Commit(ctx, []string{rel}, finalMessage, finalAuthor); err != nil {
		return err
	}
	if err := s.BuildStatic(ctx); err != nil {
		return fmt.Errorf("build static: %w", err)
	}
	return s.finalizeCommit(ctx)
}

// RenamePage moves a document and commits the rename.
func (s *Service) RenamePage(ctx context.Context, oldPath, newPath, remoteAddr string) error {
	if !s.cfg.Editable {
		return fmt.Errorf("editing disabled")
	}
	if strings.TrimSpace(newPath) == "" {
		return fmt.Errorf("new path required")
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	if err := s.ensureRepositoryFresh(ctx); err != nil {
		return err
	}

	oldRel, err := normalizeRelPath(oldPath, s.homeDoc)
	if err != nil {
		return err
	}
	newRel, err := normalizeRelPath(newPath, s.homeDoc)
	if err != nil {
		return err
	}
	if err := s.ensureRouteAccessible(oldRel); err != nil {
		return err
	}
	if err := s.ensureRouteAccessible(newRel); err != nil {
		return err
	}
	if oldRel == newRel {
		return fmt.Errorf("destination path must differ from the current path")
	}
	if isReservedPath(newRel) {
		return fmt.Errorf("%w: %s", ErrReservedPath, newRel)
	}
	if err := s.documents.Rename(ctx, oldRel, newRel); err != nil {
		return err
	}

	homeDoc := s.homeDoc
	homeDisplay := strings.TrimSuffix(filepath.ToSlash(homeDoc), filepath.Ext(homeDoc))
	if homeDisplay == "" {
		homeDisplay = "Home"
	}

	format := func(rel string) string {
		cleaned := filepath.ToSlash(rel)
		cleaned = strings.TrimSuffix(cleaned, filepath.Ext(cleaned))
		cleaned = strings.TrimPrefix(cleaned, "/")
		if cleaned == "" {
			return homeDisplay
		}
		return cleaned
	}

	message := fmt.Sprintf("Rename page: `%s` to `%s`", format(oldRel), format(newRel))
	finalMessage, err := s.composeCommitMessage(message, remoteAddr)
	if err != nil {
		return err
	}
	if err := s.documents.Commit(ctx, []string{newRel}, finalMessage, s.composeCommitAuthor("")); err != nil {
		return err
	}
	if err := s.BuildStatic(ctx); err != nil {
		return fmt.Errorf("build static: %w", err)
	}
	return s.finalizeCommit(ctx)
}

// History returns commit metadata for the provided path.
func (s *Service) History(ctx context.Context, relPath string, page, pageSize int) ([]gitutil.Commit, bool, error) {
	rel, err := normalizeRelPath(relPath, s.homeDoc)
	if err != nil {
		return nil, false, err
	}
	if err := s.ensureRouteAccessible(rel); err != nil {
		return nil, false, err
	}
	return s.documents.History(ctx, rel, page, pageSize)
}

// Diff renders a diff between two commits for the provided path.
func (s *Service) Diff(ctx context.Context, relPath, from, to string) (string, error) {
	rel, err := normalizeRelPath(relPath, s.homeDoc)
	if err != nil {
		return "", err
	}
	if err := s.ensureRouteAccessible(rel); err != nil {
		return "", err
	}
	return s.documents.Diff(ctx, rel, from, to)
}

// LoadRaw returns the underlying markdown content for editing purposes.
func (s *Service) LoadRaw(relPath string) ([]byte, error) {
	rel, err := normalizeRelPath(relPath, s.homeDoc)
	if err != nil {
		return nil, err
	}
	if err := s.ensureRouteAccessible(rel); err != nil {
		return nil, err
	}
	return s.documents.Read(rel)
}

func (s *Service) composeCommitMessage(raw, remote string) (string, error) {
	message := strings.TrimSpace(raw)
	if message == "" {
		return "", fmt.Errorf("commit message required")
	}

	if prefix := strings.TrimSpace(s.cfg.Git.CommitMessagePrefix); prefix != "" {
		message = s.cfg.Git.CommitMessagePrefix + message
	}

	if suffix := s.commitRemoteSuffix(remote); suffix != "" {
		message += suffix
	}

	if message == "" {
		return "", fmt.Errorf("commit message required")
	}
	return message, nil
}

// Use empty author to use default from config
func (s *Service) composeCommitAuthor(author string) string {
	trimmed := strings.TrimSpace(author)
	if trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(s.cfg.Git.Author)
}

func (s *Service) commitRemoteSuffix(remote string) string {
	addition := s.cfg.Git.CommitMessageAppendRemoteAddr
	if strings.TrimSpace(addition) == "" {
		return ""
	}
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return ""
	}
	if strings.Contains(addition, "%s") {
		addition = fmt.Sprintf(addition, remote)
	} else {
		addition += remote
	}
	if strings.TrimSpace(addition) == "" {
		return ""
	}
	return addition
}

func (s *Service) ensureRepositoryFresh(ctx context.Context) error {
	stale, err := s.repo.RemoteAhead(ctx)
	if err != nil {
		return err
	}
	if stale {
		return ErrRepositoryBehind
	}
	return nil
}

func (s *Service) finalizeCommit(ctx context.Context) error {
	if strings.TrimSpace(s.cfg.Git.Remote) == "" {
		return nil
	}

	stale, err := s.repo.RemoteAhead(ctx)
	if err != nil {
		return err
	}
	if stale {
		return s.rollbackWithConflict(ctx)
	}

	if err := s.repo.Push(ctx); err != nil {
		if errors.Is(err, gitutil.ErrRemoteAhead) {
			return s.rollbackWithConflict(ctx)
		}
		return err
	}
	return nil
}

func (s *Service) rollbackWithConflict(ctx context.Context) error {
	if err := s.repo.ResetSoft(ctx, "HEAD@{1}"); err != nil {
		return errors.Join(ErrRepositoryBehind, fmt.Errorf("rollback failed: %w", err))
	}
	return ErrRepositoryBehind
}
