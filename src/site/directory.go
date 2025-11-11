package site

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/iedon/dn42-wiki-go/templatex"
)

const (
	directoryPageRoute  = "/directory"
	directoryPageOutput = "directory.html"
	directoryPageTitle  = "All Pages"
)

func directoryPageHref(base string) string {
	return resolveDirectoryURL(base, directoryPageRoute)
}

func breadcrumbAnchor(segment string) string {
	segment = strings.TrimSpace(segment)
	if segment == "" {
		return ""
	}
	segment = strings.ToLower(segment)
	var b strings.Builder
	lastDash := false
	for _, r := range segment {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == ' ' || r == '-' || r == '_' || r == '.':
			if lastDash || b.Len() == 0 {
				continue
			}
			b.WriteByte('-')
			lastDash = true
		}
	}
	anchor := strings.Trim(b.String(), "-")
	return anchor
}

func (s *Service) directoryEntries(ctx context.Context) ([]*templatex.DirectoryEntry, error) {
	files, err := s.documents.ListTracked(ctx)
	if err != nil {
		return nil, err
	}

	tree := newDirectoryTree(s.cfg.BaseURL, s.homeDoc)
	for _, file := range files {
		if !isMarkdown(file) {
			continue
		}
		if isLayoutFragment(file) {
			continue
		}
		tree.add(file)
	}

	return tree.entries(), nil
}

type directoryTree struct {
	base    string
	homeDoc string
	root    *directoryNode
	anchors map[string]struct{}
}

func newDirectoryTree(base, homeDoc string) *directoryTree {
	return &directoryTree{
		base:    base,
		homeDoc: ensureHomeDoc(homeDoc),
		root:    newDirectoryNode("", "", "", "", nil),
		anchors: make(map[string]struct{}),
	}
}

func (t *directoryTree) add(relPath string) {
	slashed := filepath.ToSlash(strings.TrimSpace(relPath))
	if slashed == "" {
		return
	}
	segments := strings.Split(slashed, "/")
	if len(segments) == 0 {
		return
	}

	current := t.root
	depth := 0
	for idx, segment := range segments {
		if segment == "" {
			continue
		}
		isLast := idx == len(segments)-1
		if isLast {
			route := routeFromPath(slashed, t.homeDoc)
			if strings.EqualFold(strings.Trim(route, "/"), strings.Trim(directoryPageRoute, "/")) {
				break
			}

			title := deriveTitle(segment)
			baseSlug := normalizeAnchorCandidate(title)
			fullSlug := anchorFromRoute(route)
			id := t.allocateID(baseSlug)
			aliases := collectAliases(baseSlug, id, fullSlug)

			entry := &templatex.DirectoryEntry{
				Title:   title,
				Route:   route,
				URL:     resolveDirectoryURL(t.base, route),
				Depth:   depth + 1,
				ID:      id,
				Anchor:  baseSlug,
				Aliases: aliases,
			}
			current.documents = append(current.documents, entry)
			break
		}

		current = t.ensureChild(current, segment)
		depth++
	}
}

func (t *directoryTree) entries() []*templatex.DirectoryEntry {
	entries, _ := t.root.entries(0)
	return entries
}

type directoryNode struct {
	title     string
	route     string
	id        string
	anchor    string
	aliases   []string
	children  map[string]*directoryNode
	documents []*templatex.DirectoryEntry
}

func newDirectoryNode(title, route, id, anchor string, aliases []string) *directoryNode {
	node := &directoryNode{
		title:    title,
		route:    strings.Trim(route, "/"),
		id:       id,
		anchor:   anchor,
		aliases:  aliases,
		children: make(map[string]*directoryNode),
	}
	return node
}

func (t *directoryTree) ensureChild(parent *directoryNode, segment string) *directoryNode {
	if parent.children == nil {
		parent.children = make(map[string]*directoryNode)
	}

	key := strings.ToLower(segment)
	if child, ok := parent.children[key]; ok {
		return child
	}

	trimmed := strings.TrimSpace(segment)
	title := deriveTitle(trimmed)
	route := path.Join(parent.route, trimmed)
	baseSlug := normalizeAnchorCandidate(title)
	fullSlug := anchorFromRoute(route)
	id := t.allocateID(baseSlug)
	aliases := collectAliases(baseSlug, id, fullSlug)

	child := newDirectoryNode(title, route, id, baseSlug, aliases)
	parent.children[key] = child
	return child
}

func (n *directoryNode) entries(depth int) ([]*templatex.DirectoryEntry, int) {
	entries := make([]*templatex.DirectoryEntry, 0, len(n.children)+len(n.documents))
	total := 0

	if len(n.children) > 0 {
		keys := make([]string, 0, len(n.children))
		for key := range n.children {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool {
			return strings.ToLower(n.children[keys[i]].title) < strings.ToLower(n.children[keys[j]].title)
		})
		for _, key := range keys {
			child := n.children[key]
			childEntries, childTotal := child.entries(depth + 1)
			if len(childEntries) == 0 {
				continue
			}
			entries = append(entries, &templatex.DirectoryEntry{
				Title:    child.title,
				Children: childEntries,
				Count:    childTotal,
				Depth:    depth + 1,
				ID:       child.id,
				Anchor:   child.anchor,
				Aliases:  append([]string(nil), child.aliases...),
			})
			total += childTotal
		}
	}

	if len(n.documents) > 0 {
		sort.SliceStable(n.documents, func(i, j int) bool {
			return strings.ToLower(n.documents[i].Title) < strings.ToLower(n.documents[j].Title)
		})
		for _, doc := range n.documents {
			doc.Depth = depth + 1
		}
		entries = append(entries, n.documents...)
		total += len(n.documents)
	}

	return entries, total
}

func (t *directoryTree) allocateID(preferred string) string {
	base := strings.TrimSpace(preferred)
	if base == "" {
		base = "entry"
	}
	id := base
	suffix := 2
	for {
		if _, exists := t.anchors[id]; !exists {
			t.anchors[id] = struct{}{}
			return id
		}
		id = fmt.Sprintf("%s-%d", base, suffix)
		suffix++
	}
}

func normalizeAnchorCandidate(value string) string {
	slug := breadcrumbAnchor(value)
	if slug == "" {
		return "entry"
	}
	return slug
}

func anchorFromRoute(route string) string {
	trimmed := strings.Trim(strings.TrimSpace(route), "/")
	if trimmed == "" {
		return ""
	}
	return normalizeAnchorCandidate(strings.ReplaceAll(trimmed, "/", " "))
}

func collectAliases(values ...string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func resolveDirectoryURL(base, route string) string {
	trimmedBase := strings.Trim(strings.TrimSpace(base), "/")
	trimmedRoute := strings.TrimPrefix(strings.TrimSpace(route), "/")
	switch {
	case trimmedBase == "" && trimmedRoute == "":
		return "/"
	case trimmedBase == "":
		return "/" + trimmedRoute
	case trimmedRoute == "":
		return "/" + trimmedBase
	default:
		return "/" + path.Join(trimmedBase, trimmedRoute)
	}
}
