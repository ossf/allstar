// Copyright © 2012 Greg Jones (greg.jones@gmail.com)
// Copyright 2021 Allstar Authors

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the “Software”), to
// deal in the Software without restriction, including without limitation the
// rights to use, copy, modify, merge, publish, distribute, sublicense, and/or
// sell copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED “AS IS”, WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
// FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS
// IN THE SOFTWARE.
package ghclients

import (
	"sync"

	"github.com/rs/zerolog/log"
)

// memoryCache is an implemtation of httpcache.Cache that stores responses in
// an in-memory map.  It is a copy of httpcache.MemoryCache but adds
// LogCacheSize()
type memoryCache struct {
	mu    sync.RWMutex
	items map[string][]byte
}

// Get returns the []byte representation of the response and true if present,
// false if not
func (c *memoryCache) Get(key string) (resp []byte, ok bool) {
	c.mu.RLock()
	resp, ok = c.items[key]
	c.mu.RUnlock()

	log.Debug().
		Str("area", "bot").
		Str("key", key).
		Msg("Cache GET request")

	return resp, ok
}

// Set saves response resp to the cache with key
func (c *memoryCache) Set(key string, resp []byte) {
	c.mu.Lock()
	c.items[key] = resp
	c.mu.Unlock()

	log.Debug().
		Str("area", "bot").
		Str("key", key).
		Msg("Cache SET request")
}

// Delete removes key from the cache
func (c *memoryCache) Delete(key string) {
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()

	log.Debug().
		Str("area", "bot").
		Str("key", key).
		Msg("Cache DELETE request")
}

func (c *memoryCache) LogCacheSize() {
	var total int
	for _, b := range c.items {
		total = total + len(b)
	}
	log.Info().
		Str("area", "bot").
		Int("size", total).
		Int("items", len(c.items)).
		Msg("Total cache size.")
}

// newMemoryCache returns a new memoryCache that will store items in an
// in-memory map
func newMemoryCache() *memoryCache {
	c := &memoryCache{items: map[string][]byte{}}
	return c
}
