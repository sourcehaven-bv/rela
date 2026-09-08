package storage

import "sync"

// WriteObserver is invoked after a successful durable WriteFile,
// with the exact bytes that landed on disk after the atomic rename.
// See SafeFS.OnPostWrite.
type WriteObserver func(path string, data []byte)

// postWriteHook is the observer registry shared by SafeFS and MemFS.
// Any number of observers may be installed; each fires once per
// successful write, in installation order, and is removed only
// through the handle its installation returned. A registry rather
// than a single slot because two stores may legitimately share one FS
// (rela-desktop project switches, a future multi-project host over the
// fs build); with one slot the second store's install silently evicted
// the first's self-echo recorder (BUG-S24X52).
type postWriteHook struct {
	mu   sync.RWMutex
	next int
	obs  []postWriteEntry
}

type postWriteEntry struct {
	id int
	fn WriteObserver
}

// add installs fn and returns its removal function. A nil fn is a
// no-op whose removal does nothing. Removal is idempotent.
func (h *postWriteHook) add(fn WriteObserver) (remove func()) {
	if fn == nil {
		return func() {}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	id := h.next
	h.next++
	h.obs = append(h.obs, postWriteEntry{id: id, fn: fn})
	return func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		for i, e := range h.obs {
			if e.id == id {
				h.obs = append(h.obs[:i:i], h.obs[i+1:]...)
				return
			}
		}
	}
}

// fire invokes every installed observer with (path, data), outside
// the registry lock so an observer may install or remove observers.
func (h *postWriteHook) fire(path string, data []byte) {
	h.mu.RLock()
	snapshot := make([]WriteObserver, 0, len(h.obs))
	for _, e := range h.obs {
		snapshot = append(snapshot, e.fn)
	}
	h.mu.RUnlock()
	for _, fn := range snapshot {
		fn(path, data)
	}
}
