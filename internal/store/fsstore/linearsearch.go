package fsstore

import (
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// linearSearch is a SearchIndex that performs brute-force substring matching.
// It is used as the default when no external search index is configured.
type linearSearch struct {
	mu       sync.RWMutex
	entities map[string]*entity.Entity
}

func newLinearSearch() *linearSearch {
	return &linearSearch{entities: make(map[string]*entity.Entity)}
}

func (l *linearSearch) Index(e *entity.Entity) error {
	l.mu.Lock()
	l.entities[e.ID] = e.Clone()
	l.mu.Unlock()
	return nil
}

func (l *linearSearch) Remove(id string) error {
	l.mu.Lock()
	delete(l.entities, id)
	l.mu.Unlock()
	return nil
}

func (l *linearSearch) Search(text string, limit int) ([]string, error) {
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

func (l *linearSearch) Persistent() bool { return false }

func (l *linearSearch) Close() error {
	return nil
}
