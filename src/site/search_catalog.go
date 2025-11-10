package site

import (
	"encoding/json"
	"sync"
)

// SearchCatalog maintains the serialized search index in memory.
type SearchCatalog struct {
	mu      sync.RWMutex
	payload json.RawMessage
}

func newSearchCatalog() *SearchCatalog {
	return &SearchCatalog{}
}

func (c *SearchCatalog) Update(payload json.RawMessage) {
	c.mu.Lock()
	if len(payload) == 0 {
		c.payload = nil
	} else {
		c.payload = append(json.RawMessage(nil), payload...)
	}
	c.mu.Unlock()
}

func (c *SearchCatalog) Snapshot() json.RawMessage {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.payload) == 0 {
		return nil
	}
	clone := make(json.RawMessage, len(c.payload))
	copy(clone, c.payload)
	return clone
}
