package fsstore

import (
	"context"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Formatter returns a store.Formatter backed by this FSStore.
// The formatter re-serializes entities/relations using the canonical
// markdown format and compares to the persisted bytes, rewriting only
// when they differ.
func (s *FSStore) Formatter() store.Formatter {
	return &fsFormatter{s: s}
}

type fsFormatter struct {
	s *FSStore
}

var _ store.Formatter = (*fsFormatter)(nil)

// FormatEntity reads the persisted entity file, formats the canonical version,
// and compares. If they differ and !dryRun, it rewrites the file.
func (f *fsFormatter) FormatEntity(ctx context.Context, id string, dryRun bool) (bool, error) {
	e, err := f.s.GetEntity(ctx, id)
	if err != nil {
		return false, fmt.Errorf("get entity: %w", err)
	}

	f.s.mu.RLock()
	order := f.s.propertyOrder(e.Type)
	path := f.s.entityFilePath(e.Type, e.ID)
	f.s.mu.RUnlock()

	formatted, err := formatEntity(e, order)
	if err != nil {
		return false, fmt.Errorf("format entity: %w", err)
	}

	currentBytes, err := f.s.fs.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read entity file: %w", err)
	}

	if string(currentBytes) == formatted {
		return false, nil
	}

	if dryRun {
		return true, nil
	}

	if err := f.s.UpdateEntity(ctx, e); err != nil {
		return false, err
	}
	return true, nil
}

// FormatRelation reads the persisted relation file, formats the canonical version,
// and compares. If they differ and !dryRun, it rewrites the file.
func (f *fsFormatter) FormatRelation(ctx context.Context, from, relType, to string, dryRun bool) (bool, error) {
	r, err := f.s.GetRelation(ctx, from, relType, to)
	if err != nil {
		return false, fmt.Errorf("get relation: %w", err)
	}

	f.s.mu.RLock()
	path := f.s.relationFilePath(from, relType, to)
	f.s.mu.RUnlock()

	formatted, err := formatRelation(r)
	if err != nil {
		return false, fmt.Errorf("format relation: %w", err)
	}

	currentBytes, err := f.s.fs.ReadFile(path)
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
	if _, err := f.s.UpdateRelation(ctx, from, relType, to, data); err != nil {
		return false, err
	}
	return true, nil
}
