package visibility

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// ExternalRefHolder names one (type, property) declaring an external-ref
// system. The caller derives the list from the metamodel, which visibility
// may not import.
type ExternalRefHolder struct {
	Type     string
	Property string
	// Sync reports `sync: true` on the property.
	Sync bool
}

// ErrSyncNeedsHistory refuses a lookup of a `sync: true` external ref on a
// reader without version history and tags (TKT-SM20FG): a sync connector
// cannot merge without its base, so it must not start.
var ErrSyncNeedsHistory = fmt.Errorf("sync needs version history: %w", store.ErrHistoryUnsupported)

// FindByExternalRef returns the entities the caller may read whose external
// ref, in any of holders, has id (TKT-SM20FG). holders are every (type,
// property) declaring one system.
//
// The candidates come from one raw header query per holder, so the lookup
// is an index probe and not a scan. Each candidate (id, face) is then read
// through the gated resolver at that face, the one holding the ref, and
// kept only when the redacted row still carries id. So a hidden entity, a
// hidden face and a hidden ref property all count as no match, and the
// result is no oracle for them. An entity whose faces share the id is
// returned once, at the first readable face holding it. A caller that gets
// nothing must not conclude the id is free: the write path's unique check
// sees every holder.
func (s *ScriptReader) FindByExternalRef(
	ctx context.Context, holders []ExternalRefHolder, id string,
) ([]*entity.Entity, error) {
	if err := checkSyncHistory(holders, s.history, s.tags); err != nil {
		return nil, err
	}
	ctx = s.bind(ctx)
	return findByExternalRef(ctx, s.raw, holders, id, func(ref entity.Ref) (*entity.Entity, error) {
		return s.res.addressAny(ctx, worldIn(ctx, s.world), ref.String())
	})
}

// FindByExternalRef is [ScriptReader.FindByExternalRef] without the gate or
// the redaction.
func (r *UnrestrictedReader) FindByExternalRef(
	ctx context.Context, holders []ExternalRefHolder, id string,
) ([]*entity.Entity, error) {
	if err := checkSyncHistory(holders, r.history, r.tags); err != nil {
		return nil, err
	}
	return findByExternalRef(ctx, r.st, holders, id, func(ref entity.Ref) (*entity.Entity, error) {
		return r.GetAddress(ctx, ref.String())
	})
}

// FindByExternalRef finds nothing, like every DenyReader read.
func (DenyReader) FindByExternalRef(context.Context, []ExternalRefHolder, string) ([]*entity.Entity, error) {
	return nil, nil
}

// checkSyncHistory refuses a sync holder on a reader without history.
func checkSyncHistory(holders []ExternalRefHolder, hr store.HistoryReader, tags VersionTagReader) error {
	for _, h := range holders {
		if h.Sync && (hr == nil || tags == nil) {
			return ErrSyncNeedsHistory
		}
	}
	return nil
}

// findByExternalRef is the shared body: raw candidates per (id, face),
// then a read of each at its own face.
func findByExternalRef(
	ctx context.Context, raw store.Store, holders []ExternalRefHolder, id string,
	read func(ref entity.Ref) (*entity.Entity, error),
) ([]*entity.Entity, error) {
	if id == "" {
		return nil, nil
	}
	found := map[string]bool{}
	var out []*entity.Entity
	for _, h := range holders {
		q := store.GraphQuery{
			EntityType: h.Type,
			Props: []store.PropPredicate{{
				Property: h.Property, Op: store.PropKeyEqual, Key: "id", Value: id,
			}},
			Faces: store.AllFaces(),
		}
		var refs []entity.Ref
		for hdr, err := range store.GraphQueryHeaders(ctx, raw, q) {
			if err != nil {
				return nil, fmt.Errorf("visibility: find external ref: %w", err)
			}
			refs = append(refs, entity.Ref{ID: hdr.ID, Face: hdr.Face})
		}
		for _, ref := range refs {
			if found[ref.ID] {
				continue
			}
			e, err := read(ref)
			if errors.Is(err, store.ErrNotFound) {
				continue
			}
			if err != nil {
				// An explicit face never picks among faces, so an
				// AmbiguousAddressError here is a bug, not a miss.
				return nil, fmt.Errorf("visibility: find external ref: %w", err)
			}
			if refID(e.Properties[h.Property]) == id {
				found[ref.ID] = true
				out = append(out, e)
			}
		}
	}
	return out, nil
}

// refID is the string `id` entry of an external-ref value, or "". The
// value was validated on write; anything else is no match.
func refID(v any) string {
	switch m := v.(type) {
	case map[string]any:
		s, _ := m["id"].(string)
		return s
	case map[string]string:
		return m["id"]
	}
	return ""
}

// SyncReady reports whether s serves version history and tags, which a sync
// merge needs for its base.
func (s *ScriptReader) SyncReady() bool { return s.history != nil && s.tags != nil }

// SyncReady is [ScriptReader.SyncReady] for the ungated reader.
func (r *UnrestrictedReader) SyncReady() bool { return r.history != nil && r.tags != nil }
