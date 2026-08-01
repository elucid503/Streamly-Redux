package subdl

import (
	"sync"
)

const maxEntries = 200

type cache struct {

	mu sync.Mutex
	entries map[string][]byte

}

func newCache() *cache {

	return &cache{

		entries: map[string][]byte{},

	}

}

func (c *cache) get(key string) ([]byte, bool) {

	c.mu.Lock()

	defer c.mu.Unlock()

	value, ok := c.entries[key]

	return value, ok

}

func (c *cache) put(key string, value []byte) {

	c.mu.Lock()

	defer c.mu.Unlock()

	// Tracks are small and a session is short, so eviction only needs to stop unbounded growth.
	if len(c.entries) >= maxEntries {

		for existing := range c.entries {

			delete(c.entries, existing)
			break

		}

	}

	c.entries[key] = value

}
