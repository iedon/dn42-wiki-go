package site

import (
	"html/template"
	"sync"
	"time"
)

// LayoutSnapshot holds the cached header/footer/sidebar fragments.
type LayoutSnapshot struct {
	Header       template.HTML
	Footer       template.HTML
	ServerFooter template.HTML
	Sidebar      template.HTML
	LoadedAt     time.Time
}

type LayoutCache struct {
	mu       sync.RWMutex
	snapshot LayoutSnapshot
}

func newLayoutCache() *LayoutCache {
	return &LayoutCache{}
}

func (c *LayoutCache) Update(header, footer, serverFooter, sidebar template.HTML) {
	c.mu.Lock()
	c.snapshot = LayoutSnapshot{
		Header:       header,
		Footer:       footer,
		ServerFooter: serverFooter,
		Sidebar:      sidebar,
		LoadedAt:     time.Now(),
	}
	c.mu.Unlock()
}

func (c *LayoutCache) Snapshot() LayoutSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.snapshot
}
