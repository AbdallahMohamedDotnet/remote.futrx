package agentcatalog

import (
	"sync"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
)

const cacheTTL = 30 * time.Second

type catalogCache struct {
	mu      sync.Mutex
	now     func() time.Time
	entries map[string]cacheEntry
}

type cacheEntry struct {
	expiresAt time.Time
	result    []agent.Capabilities
}

func newCatalogCache() *catalogCache {
	return &catalogCache{
		now:     time.Now,
		entries: make(map[string]cacheEntry),
	}
}

func (c *catalogCache) load(key string) ([]agent.Capabilities, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok || !c.now().Before(entry.expiresAt) {
		delete(c.entries, key)
		return nil, false
	}
	return cloneCapabilities(entry.result), true
}

func (c *catalogCache) store(key string, result []agent.Capabilities) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = cacheEntry{
		expiresAt: c.now().Add(cacheTTL),
		result:    cloneCapabilities(result),
	}
}

func cloneCapabilities(input []agent.Capabilities) []agent.Capabilities {
	output := make([]agent.Capabilities, len(input))
	for index, caps := range input {
		output[index] = caps
		output[index].Modes = append([]agent.CapabilityOption(nil), caps.Modes...)
		output[index].Models = make([]agent.ModelCapability, len(caps.Models))
		for modelIndex, model := range caps.Models {
			output[index].Models[modelIndex] = model
			output[index].Models[modelIndex].ReasoningEfforts = append(
				[]agent.CapabilityOption(nil), model.ReasoningEfforts...,
			)
			output[index].Models[modelIndex].ServiceTiers = append(
				[]agent.CapabilityOption(nil), model.ServiceTiers...,
			)
		}
	}
	return output
}
