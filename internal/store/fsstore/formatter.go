package fsstore

import (
	"context"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// FSStore satisfies store.Formatter directly.
var _ store.Formatter = (*FSStore)(nil)

// FormatEntity reads the persisted entity file, formats the canonical version,
// and compares. If they differ and !dryRun, it rewrites the file.
func (s *FSStore) FormatEntity(ctx context.Context, id string, dryRun bool) (bool, error) {
	e, err := s.GetEntity(ctx, id)
	if err != nil {
		return false, fmt.Errorf("get entity: %w", err)
	}

	s.mu.RLock()
	order := s.propertyOrder(e.Type)
	path := s.entityFilePath(e.Type, e.ID)
	s.mu.RUnlock()

	formatted, err := formatEntity(e, order, s.crypto)
	if err != nil {
		return false, fmt.Errorf("format entity: %w", err)
	}

	currentBytes, err := s.fs.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read entity file: %w", err)
	}

	if string(currentBytes) == formatted {
		return false, nil
	}

	if dryRun {
		return true, nil
	}

	if err := s.UpdateEntity(ctx, e); err != nil {
		return false, err
	}
	return true, nil
}

// FormatRelation reads the persisted relation file, formats the canonical version,
// and compares. If they differ and !dryRun, it rewrites the file.
func (s *FSStore) FormatRelation(ctx context.Context, from, relType, to string, dryRun bool) (bool, error) {
	r, err := s.GetRelation(ctx, from, relType, to)
	if err != nil {
		return false, fmt.Errorf("get relation: %w", err)
	}

	s.mu.RLock()
	path := s.relationFilePath(from, relType, to)
	s.mu.RUnlock()

	formatted, err := formatRelation(r)
	if err != nil {
		return false, fmt.Errorf("format relation: %w", err)
	}

	currentBytes, err := s.fs.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read relation file: %w", err)
	}

	if string(currentBytes) == formatted {
		return false, nil
	}

	if dryRun {
		return true, nil
	}

	data := store.RelationData{Content: r.Content, Properties: r.Properties}
	if _, err := s.UpdateRelation(ctx, from, relType, to, data); err != nil {
		return false, err
	}
	return true, nil
}
