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
func (s *FSStore) loadEntity(id, entityType string) (*entity.Entity, error) {
	path := s.entityFilePath(entityType, id)
	return s.readEntityFile(path)
}

// loadRelation reads a single relation from disk.
func (s *FSStore) loadRelation(from, relType, to string) (*entity.Relation, error) {
	path := s.relationFilePath(from, relType, to)
	return s.readRelationFile(path)
}
