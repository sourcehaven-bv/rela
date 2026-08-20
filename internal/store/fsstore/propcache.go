package fsstore

import (
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// addEntityToCache adds property values from an entity to the cache.
func addEntityToCache(cache map[string]map[string]int, e *entity.Entity) {
	for k, v := range e.Properties {
		s := fmt.Sprintf("%v", v)
		if s == "" {
			continue
		}
		if cache[k] == nil {
			cache[k] = make(map[string]int)
		}
		cache[k][s]++
	}
}

// removeEntityFromCache decrements property values in the cache.
func removeEntityFromCache(cache map[string]map[string]int, e *entity.Entity) {
	for k, v := range e.Properties {
		s := fmt.Sprintf("%v", v)
		if s == "" {
			continue
		}
		if vals, ok := cache[k]; ok {
			vals[s]--
			if vals[s] <= 0 {
				delete(vals, s)
			}
			if len(vals) == 0 {
				delete(cache, k)
			}
		}
	}
}

// loadEntity reads a single entity from disk.
// loadEntityMeta reads the state an index meta describes. The pointer is
// filename/index-authoritative — entity frontmatter carries no pointer
// key — so it is stamped here after the read (TKT-DOFYR1).
func (s *FSStore) loadEntityMeta(m entityMeta) (*entity.Entity, error) {
	key := s.entityFileKey(m.Type, stateKey(m.ID, m.Pointer))
	e, err := s.readEntityFile(key, m.ID, m.Type)
	if err != nil {
		return nil, err
	}
	e.Pointer = m.Pointer
	return e, nil
}

// loadRelation reads a single relation from disk.
func (s *FSStore) loadRelation(from, relType, to string) (*entity.Relation, error) {
	return s.readRelationFile(s.relationFileKey(from, relType, to), from, relType, to)
}

// loadRelationMeta reads the relation an index meta describes, stamping
// the identity-bearing tail pointer from the index.
func (s *FSStore) loadRelationMeta(m relationMeta) (*entity.Relation, error) {
	r, err := s.readRelationFile(s.relationFileKeyMeta(m), m.From, m.Type, m.To)
	if err != nil {
		return nil, err
	}
	r.FromPointer = m.FromPointer
	return r, nil
}
