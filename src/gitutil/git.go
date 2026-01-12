package gitutil

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Repository represents a cloned git repository and offers limited VCS operations.
type Repository struct {
	Dir            string
	Remote         string
	GitPath        string
	CommandTimeout time.Duration
	mu             sync.Mutex
}

// ErrRemoteAhead indicates the upstream repository contains commits the
// local clone has not incorporated yet.
var ErrRemoteAhead = errors.New("remote contains newer commits")

// Commit encapsulates log metadata for UI consumption.
type Commit struct {
	Hash        string    `json:"hash"`
	Author      string    `json:"author"`
	Email       string    `json:"email"`
	Message     string    `json:"message"`
	CommittedAt time.Time `json:"committedAt"`
}

// NewRepository ensures the repository exists locally by cloning if needed.
func NewRepository(gitPath, remote, dir string, timeout time.Duration) (*Repository, error) {
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	repo := &Repository{Dir: dir, Remote: remote, GitPath: gitPath, CommandTimeout: timeout}
	if err := repo.ensureClone(); err != nil {
		return nil, err
	}
	return repo, nil
}

// Pull updates the repository with remote changes.
func (r *Repository) Pull(ctx context.Context) (bool, error) {
	if strings.TrimSpace(r.Remote) == "" {
		return false, nil
	}

	ctx, cancel := r.ensureContext(ctx)
	defer cancel()

	r.mu.Lock()
	defer r.mu.Unlock()

	prev, prevErr := r.headHash(ctx)

	cmd := r.command(ctx, "pull", "--ff-only")
	if prevErr != nil {
		return false, prevErr
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		outStr := string(out)
		if bytes.Contains(out, []byte("You have not concluded your merge")) {
			return false, fmt.Errorf("pull aborted: %s", out)
		}
		if needsRebaseFallback(outStr) {
			if err := r.pullWithRebase(ctx); err != nil {
				return false, err
			}
			after, afterErr := r.headHash(ctx)
			if afterErr != nil {
				return false, afterErr
			}
			return after != prev, nil
		}
		return false, fmt.Errorf("git pull: %w (%s)", err, outStr)
	}
	after, afterErr := r.headHash(ctx)
	if afterErr != nil {
		return false, afterErr
	}
	return after != prev, nil
}

func (r *Repository) pullWithRebase(ctx context.Context) error {
	cmd := r.command(ctx, "pull", "--rebase")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git pull --rebase: %w (%s)", err, string(out))
	}
	return nil
}

// RemoteAhead reports whether the upstream branch contains commits that are
// not present locally.
func (r *Repository) RemoteAhead(ctx context.Context) (bool, error) {
	if strings.TrimSpace(r.Remote) == "" {
		return false, nil
	}

	ctx, cancel := r.ensureContext(ctx)
	defer cancel()

	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.fetchLocked(ctx); err != nil {
		return false, err
	}
	return r.remoteAheadLocked(ctx)
}

func (r *Repository) headHash(ctx context.Context) (string, error) {
	cmd := r.command(ctx, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func needsRebaseFallback(output string) bool {
	markers := []string{
		"Not possible to fast-forward",
		"cannot fast-forward",
		"Diverging branches",
	}
	for _, marker := range markers {
		if strings.Contains(output, marker) {
			return true
		}
	}
	return false
}

func isNonFastForward(output string) bool {
	markers := []string{
		"non-fast-forward",
		"fetch first",
		"Updates were rejected because",
		"failed to push some refs",
	}
	lowered := strings.ToLower(output)
	for _, marker := range markers {
		if strings.Contains(lowered, strings.ToLower(marker)) {
			return true
		}
	}
	return false
}

// PullPath triggers git fetch specifically for a file path to warm caches.
func (r *Repository) PullPath(ctx context.Context, path string) error {
	_, err := r.Pull(ctx)
	return err
}

// Log returns paginated commit history scoped to a file path.
func (r *Repository) Log(ctx context.Context, path string, page, pageSize int) ([]Commit, bool, error) {
	ctx, cancel := r.ensureContext(ctx)
	defer cancel()

	r.mu.Lock()
	defer r.mu.Unlock()

	offset := page * pageSize
	args := []string{"log", fmt.Sprintf("--skip=%d", offset), fmt.Sprintf("-n%d", pageSize+1), "--date=unix", "--pretty=%H%x00%an%x00%ae%x00%at%x00%s"}
	if path != "" {
		args = append(args, "--", filepath.ToSlash(path))
	}
	cmd := r.command(ctx, args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, false, fmt.Errorf("git log: %w", err)
	}

	lines := bytes.Split(bytes.TrimSpace(out), []byte("\n"))
	hasMore := false
	if len(lines) > pageSize {
		hasMore = true
		lines = lines[:pageSize]
	}

	commits := make([]Commit, 0, len(lines))
	for _, ln := range lines {
		parts := bytes.Split(ln, []byte{0})
		if len(parts) != 5 {
			continue
		}
		seconds, err := parseUnix(parts[3])
		if err != nil {
			return nil, false, err
		}
		commits = append(commits, Commit{
			Hash:        string(parts[0]),
			Author:      string(parts[1]),
			Email:       string(parts[2]),
			CommittedAt: time.Unix(seconds, 0).UTC(),
			Message:     string(parts[4]),
		})
	}

	return commits, hasMore, nil
}

// Diff renders a colored diff between two commits for a path.
func (r *Repository) Diff(ctx context.Context, path, from, to string) (string, error) {
	ctx, cancel := r.ensureContext(ctx)
	defer cancel()

	r.mu.Lock()
	defer r.mu.Unlock()

	if from == "" || to == "" {
		return "", errors.New("from and to commit hashes are required")
	}
	args := []string{"diff", fmt.Sprintf("%s..%s", from, to), "--", filepath.ToSlash(path)}
	cmd := r.command(ctx, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git diff: %w (%s)", err, string(out))
	}
	return string(out), nil
}

// ReadFile reads repository content at HEAD.
func (r *Repository) ReadFile(path string) ([]byte, error) {
	full := filepath.Join(r.Dir, filepath.FromSlash(path))
	return os.ReadFile(full)
}

// WriteFile writes to a file inside the repository.
func (r *Repository) WriteFile(path string, data []byte) error {
	full := filepath.Join(r.Dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, data, 0o644)
}

// Rename moves a file to the desired destination using git mv.
func (r *Repository) Rename(ctx context.Context, oldPath, newPath string) error {
	ctx, cancel := r.ensureContext(ctx)
	defer cancel()

	r.mu.Lock()
	defer r.mu.Unlock()

	cmd := r.command(ctx, "mv", filepath.ToSlash(oldPath), filepath.ToSlash(newPath))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git mv: %w (%s)", err, string(out))
	}
	return nil
}

// Push propagates local commits to the remote.
func (r *Repository) Push(ctx context.Context) error {
	if strings.TrimSpace(r.Remote) == "" {
		return nil
	}

	ctx, cancel := r.ensureContext(ctx)
	defer cancel()

	r.mu.Lock()
	defer r.mu.Unlock()

	cmd := r.command(ctx, "push")
	if out, err := cmd.CombinedOutput(); err != nil {
		outStr := string(out)
		if isNonFastForward(outStr) {
			return errors.Join(ErrRemoteAhead, fmt.Errorf("git push rejected: %s", strings.TrimSpace(outStr)))
		}
		return fmt.Errorf("git push: %w (%s)", err, outStr)
	}
	return nil
}

// CommitChanges stages and commits files with provided message.
func (r *Repository) CommitChanges(ctx context.Context, paths []string, message string, author string) error {
	if strings.TrimSpace(message) == "" {
		return errors.New("commit message required")
	}

	ctx, cancel := r.ensureContext(ctx)
	defer cancel()

	r.mu.Lock()
	defer r.mu.Unlock()

	sanitized := normalizePaths(paths)
	stageArgs := []string{"add"}
	if len(sanitized) == 0 {
		stageArgs = append(stageArgs, "--all")
	} else {
		stageArgs = append(stageArgs, "--")
		stageArgs = append(stageArgs, sanitized...)
	}
	cmd := r.command(ctx, stageArgs...)
	if out, err := cmd.CombinedOutput(); err != nil {
		outStr := strings.TrimSpace(string(out))
		if len(sanitized) > 0 && (strings.Contains(outStr, "did not match any files") || strings.Contains(outStr, "pathspec")) {
			fallback := []string{"add", "--update", "--"}
			fallback = append(fallback, sanitized...)
			cmd = r.command(ctx, fallback...)
			if retryOut, retryErr := cmd.CombinedOutput(); retryErr != nil {
				return fmt.Errorf("git add: %w (%s)", retryErr, strings.TrimSpace(string(retryOut)))
			}
		} else {
			if outStr == "" {
				outStr = err.Error()
			}
			return fmt.Errorf("git add: %w (%s)", err, outStr)
		}
	}

	commitArgs := []string{"commit", "-m", message}
	if author != "" {
		commitArgs = append(commitArgs, "--author", author)
	}
	cmd = r.command(ctx, commitArgs...)
	if out, err := cmd.CombinedOutput(); err != nil {
		outStr := string(out)
		if strings.Contains(outStr, "nothing added to commit") {
			if err := r.stageAll(ctx); err != nil {
				return fmt.Errorf("git commit: %w", err)
			}
			cmd = r.command(ctx, commitArgs...)
			if retryOut, retryErr := cmd.CombinedOutput(); retryErr != nil {
				return fmt.Errorf("git commit: %w (%s)", retryErr, string(retryOut))
			}
			return nil
		}
		return fmt.Errorf("git commit: %w (%s)", err, outStr)
	}
	return nil
}

func (r *Repository) stageAll(ctx context.Context) error {
	cmd := r.command(ctx, "add", "--all")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add: %w (%s)", err, string(out))
	}
	return nil
}

// ListTrackedFiles returns all tracked files.
func (r *Repository) ListTrackedFiles(ctx context.Context) ([]string, error) {
	ctx, cancel := r.ensureContext(ctx)
	defer cancel()

	r.mu.Lock()
	defer r.mu.Unlock()

	cmd := r.command(ctx, "ls-files")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return []string{}, nil
	}
	return lines, nil
}

func (r *Repository) ensureClone() error {
	if _, err := os.Stat(filepath.Join(r.Dir, ".git")); err == nil {
		return nil
	}

	if err := os.MkdirAll(r.Dir, 0o755); err != nil {
		return err
	}

	if strings.TrimSpace(r.Remote) == "" {
		ctx, cancel := context.WithTimeout(context.Background(), r.CommandTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, r.GitPath, "init")
		cmd.Dir = r.Dir
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git init: %w (%s)", err, string(out))
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), r.CommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, r.GitPath, "clone", r.Remote, r.Dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone: %w (%s)", err, string(out))
	}
	return nil
}

func (r *Repository) command(ctx context.Context, args ...string) *exec.Cmd {
	if ctx == nil {
		ctx = context.Background()
	}

	baseArgs := []string{
		"-c", "credential.helper=", // Disable credential helper to prevent daemon spawning
	}
	fullArgs := append(baseArgs, args...)

	cmd := exec.CommandContext(ctx, r.GitPath, fullArgs...)
	cmd.Dir = r.Dir
	return cmd
}

func (r *Repository) ensureContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx != nil {
		return ctx, func() {}
	}
	ctx, cancel := context.WithTimeout(context.Background(), r.CommandTimeout)
	return ctx, cancel
}

func normalizePaths(paths []string) []string {
	result := make([]string, 0, len(paths))
	for _, p := range paths {
		if strings.TrimSpace(p) == "" {
			continue
		}
		result = append(result, filepath.ToSlash(p))
	}
	return result
}

func parseUnix(raw []byte) (int64, error) {
	return strconv.ParseInt(string(raw), 10, 64)
}

func (r *Repository) fetchLocked(ctx context.Context) error {
	cmd := r.command(ctx, "fetch", "--quiet")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (r *Repository) remoteAheadLocked(ctx context.Context) (bool, error) {
	cmd := r.command(ctx, "rev-list", "--left-right", "--count", "HEAD...@{u}")
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))
	if err != nil {
		if output == "" {
			output = err.Error()
		}
		if strings.Contains(output, "no upstream configured") || strings.Contains(output, "missing upstream") || strings.Contains(output, "does not point to a branch") {
			return false, nil
		}
		return false, fmt.Errorf("git rev-list: %w (%s)", err, output)
	}
	if output == "" {
		return false, nil
	}
	fields := strings.Fields(output)
	if len(fields) < 2 {
		return false, fmt.Errorf("git rev-list: unexpected output: %q", output)
	}
	remoteAhead, convErr := strconv.Atoi(fields[1])
	if convErr != nil {
		return false, fmt.Errorf("git rev-list: parse output: %w", convErr)
	}
	return remoteAhead > 0, nil
}

// ResetSoft rewinds HEAD while preserving staged and working tree changes.
func (r *Repository) ResetSoft(ctx context.Context, target string) error {
	if strings.TrimSpace(target) == "" {
		return errors.New("reset target required")
	}

	ctx, cancel := r.ensureContext(ctx)
	defer cancel()

	r.mu.Lock()
	defer r.mu.Unlock()

	cmd := r.command(ctx, "reset", "--soft", target)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git reset --soft %s: %w (%s)", target, err, strings.TrimSpace(string(out)))
	}
	return nil
}
