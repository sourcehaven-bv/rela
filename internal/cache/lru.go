// Package cache provides small generic cache data structures.
package cache

import (
	"container/list"
	"sync"
)

// LRU is a thread-safe bounded least-recently-used cache.
//
// When the cache is full, Put evicts the least-recently-used entry. Both
// Get and Put count as "use" — they move the touched entry to the front
// of the recency list. A capacity of 0 or less is treated as 1.
type LRU[K comparable, V any] struct {
	mu       sync.Mutex
	capacity int
	items    map[K]*list.Element
	order    *list.List // front = most recently used
}

type lruEntry[K comparable, V any] struct {
	key   K
	value V
}

// NewLRU creates a new LRU cache with the given capacity.
func NewLRU[K comparable, V any](capacity int) *LRU[K, V] {
	if capacity < 1 {
		capacity = 1
	}
	return &LRU[K, V]{
		capacity: capacity,
		items:    make(map[K]*list.Element, capacity),
		order:    list.New(),
	}
}

// Get returns the value for key and whether it was found. A hit moves
// the entry to the most-recently-used position.
func (c *LRU[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.items[key]; ok {
		c.order.MoveToFront(el)
		entry, _ := el.Value.(*lruEntry[K, V])
		return entry.value, true
	}
	var zero V
	return zero, false
}

// Put inserts or updates the value for key, moving it to the
// most-recently-used position. If insertion exceeds the capacity, the
// least-recently-used entry is evicted.
func (c *LRU[K, V]) Put(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.items[key]; ok {
		entry, _ := el.Value.(*lruEntry[K, V])
		entry.value = value
		c.order.MoveToFront(el)
		return
	}

	el := c.order.PushFront(&lruEntry[K, V]{key: key, value: value})
	c.items[key] = el

	if c.order.Len() > c.capacity {
		oldest := c.order.Back()
		if oldest != nil {
			c.order.Remove(oldest)
			entry, _ := oldest.Value.(*lruEntry[K, V])
			delete(c.items, entry.key)
		}
	}
}

// Delete removes the entry for key, if any.
func (c *LRU[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.items[key]; ok {
		c.order.Remove(el)
		delete(c.items, key)
	}
}

// Len returns the number of entries currently in the cache.
func (c *LRU[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}
