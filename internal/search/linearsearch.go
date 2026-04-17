package search

import (
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// LinearSearch is a SearchIndex that performs brute-force substring matching.
// It is a simple in-memory fallback for backends that don't have a dedicated
// full-text index.
type LinearSearch struct {
	mu           sync.RWMutex
	entities     map[string]*entity.Entity
	lastModified time.Time
}

// NewLinearSearch creates a new linear substring search index.
func NewLinearSearch() *LinearSearch {
	return &LinearSearch{entities: make(map[string]*entity.Entity)}
}

func (l *LinearSearch) EntityPut(e *entity.Entity) error {
	l.mu.Lock()
	l.entities[e.ID] = e.Clone()
	l.advanceLastModified(e.UpdatedAt)
	l.mu.Unlock()
	return nil
}

func (l *LinearSearch) EntityDelete(id string) error {
	l.mu.Lock()
	delete(l.entities, id)
	// A delete carries no mtime from the entity; use wall clock so the
	// timestamp still advances and consumers can observe the change.
	l.advanceLastModified(time.Now())
	l.mu.Unlock()
	return nil
}

// LastModified returns the latest mtime observed by this index across all
// EntityPut and EntityDelete calls. Consumers compare this against the
// store's LastModified to decide whether the index needs repopulating.
func (l *LinearSearch) LastModified() time.Time {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.lastModified
}

// advanceLastModified bumps the cursor forward; callers hold mu.
func (l *LinearSearch) advanceLastModified(t time.Time) {
	if t.After(l.lastModified) {
		l.lastModified = t
	}
}

func (l *LinearSearch) Search(text string, limit int) ([]string, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var ids []string
	for _, e := range l.entities {
		if storeutil.MatchText(e, text) {
			ids = append(ids, e.ID)
			if limit > 0 && len(ids) >= limit {
				break
			}
		}
	}
	return ids, nil
}

func (l *LinearSearch) Close() error {
	return nil
}
