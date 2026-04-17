package fsstore

import (
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// LinearSearch is a SearchIndex that performs brute-force substring matching.
// It is used as the default when no external search index is configured.
type LinearSearch struct {
	mu       sync.RWMutex
	entities map[string]*entity.Entity
}

// NewLinearSearch creates a new linear substring search index.
func NewLinearSearch() *LinearSearch {
	return &LinearSearch{entities: make(map[string]*entity.Entity)}
}

func (l *LinearSearch) EntityPut(e *entity.Entity) error {
	l.mu.Lock()
	l.entities[e.ID] = e.Clone()
	l.mu.Unlock()
	return nil
}

func (l *LinearSearch) EntityDelete(id string) error {
	l.mu.Lock()
	delete(l.entities, id)
	l.mu.Unlock()
	return nil
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

func (l *LinearSearch) Persistent() bool { return false }

func (l *LinearSearch) Close() error {
	return nil
}
