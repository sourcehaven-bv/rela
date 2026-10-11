package visibility

import (
	"context"
	"maps"

	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// WithHistory returns a copy of s that serves version history from h, the
// store's history capability. Nil, or never called, leaves s without history:
// both history reads answer [store.ErrHistoryUnsupported].
//
// History is a separate handle, not a type assertion on the raw store,
// because a backend exposes it beside the store (its VersionStore), not as a
// method set of the store itself.
func (s *ScriptReader) WithHistory(h store.HistoryReader) *ScriptReader {
	c := *s
	c.history = h
	return &c
}

// WithHistory is [ScriptReader.WithHistory] for the ungated reader.
func (r *UnrestrictedReader) WithHistory(h store.HistoryReader) *UnrestrictedReader {
	c := *r
	c.history = h
	return &c
}

// EntityVersions returns the version timeline of the entity face addr names,
// oldest first, for a caller who may read that face now (TKT-EC7F65).
//
// The face is resolved by the same gated read as [ScriptReader.GetAddress],
// so a hidden, missing or deleted entity is [store.ErrNotFound], and the
// timeline is the lineage of the face that read served. A store that keeps no
// history answers [store.ErrHistoryUnsupported] before the address is looked
// at, so the answer does not depend on whether the entity exists.
func (s *ScriptReader) EntityVersions(ctx context.Context, addr string) ([]store.VersionMeta, error) {
	ctx = s.bind(ctx)
	return entityVersions(ctx, s.history, func() (*entity.Entity, error) {
		return s.res.addressAny(ctx, worldIn(ctx, s.world), addr)
	})
}

// EntityVersion returns the entity face addr names as it was at version n,
// with that version's metadata. A purge row, which keeps no content, is
// [store.ErrNotFound]. Gating is that of [ScriptReader.EntityVersions];
// the snapshot is then redacted for the caller with the historical-subject
// marker (TKT-73C6B2), so a conditional `visible:` grant whose inputs the
// live graph cannot vouch for hides its field. A version that does not exist
// is [store.ErrNotFound].
func (s *ScriptReader) EntityVersion(
	ctx context.Context, addr string, n int,
) (*entity.Entity, store.VersionMeta, error) {
	ctx = s.bind(ctx)
	e, meta, err := entityVersion(ctx, s.history, n, func() (*entity.Entity, error) {
		return s.res.addressAny(ctx, worldIn(ctx, s.world), addr)
	})
	if err != nil {
		return nil, store.VersionMeta{}, err
	}
	red, ok := s.reader.(rowRedactor)
	if !ok {
		// Every Reader this package builds can redact a row. One that cannot
		// serves no snapshot, rather than an unredacted one.
		return nil, store.VersionMeta{}, store.ErrNotFound
	}
	return red.RedactRow(affordances.WithHistoricalSubject(ctx), e), meta, nil
}

// EntityVersions is [ScriptReader.EntityVersions] without the gate.
func (r *UnrestrictedReader) EntityVersions(ctx context.Context, addr string) ([]store.VersionMeta, error) {
	return entityVersions(ctx, r.history, func() (*entity.Entity, error) {
		return r.GetAddress(ctx, addr)
	})
}

// EntityVersion is [ScriptReader.EntityVersion] without the gate or the
// redaction.
func (r *UnrestrictedReader) EntityVersion(
	ctx context.Context, addr string, n int,
) (*entity.Entity, store.VersionMeta, error) {
	return entityVersion(ctx, r.history, n, func() (*entity.Entity, error) {
		return r.GetAddress(ctx, addr)
	})
}

// EntityVersions denies, like every DenyReader read. A miss rather than
// [store.ErrHistoryUnsupported]: the refusal must look like the reads beside
// it, and a miss is never mistaken for an empty timeline.
func (DenyReader) EntityVersions(context.Context, string) ([]store.VersionMeta, error) {
	return nil, store.ErrNotFound
}

// EntityVersion denies, like every DenyReader read.
func (DenyReader) EntityVersion(context.Context, string, int) (*entity.Entity, store.VersionMeta, error) {
	return nil, store.VersionMeta{}, store.ErrNotFound
}

// entityVersions reads the timeline of the live face live returns.
func entityVersions(
	ctx context.Context, hr store.HistoryReader, live func() (*entity.Entity, error),
) ([]store.VersionMeta, error) {
	if hr == nil {
		return nil, store.ErrHistoryUnsupported
	}
	e, err := live()
	if err != nil {
		return nil, err
	}
	return hr.ListVersions(ctx, e.Ref())
}

// entityVersion reads version n of the live face live returns, unredacted,
// through [snapshotEntity].
func entityVersion(
	ctx context.Context, hr store.HistoryReader, n int, live func() (*entity.Entity, error),
) (*entity.Entity, store.VersionMeta, error) {
	if hr == nil {
		return nil, store.VersionMeta{}, store.ErrHistoryUnsupported
	}
	cur, err := live()
	if err != nil {
		return nil, store.VersionMeta{}, err
	}
	snap, err := hr.GetVersion(ctx, cur.Ref(), n)
	if err != nil {
		return nil, store.VersionMeta{}, err
	}
	return snapshotEntity(cur, snap)
}

// snapshotEntity serves snap as the entity cur was at that version,
// unredacted. A snapshot of another type or face than the live row is
// refused: the gate ran for the live row, so serving another type's lineage
// under it is the cross-type leak the HTTP history API refuses too.
func snapshotEntity(cur *entity.Entity, snap *store.VersionSnapshot) (*entity.Entity, store.VersionMeta, error) {
	if snap.Type != cur.Type || snap.Face != cur.Face {
		return nil, store.VersionMeta{}, store.ErrNotFound
	}
	// A purge row keeps no content. Served, it would read as an entity whose
	// every field is empty, which a caller comparing against it would take
	// for real values.
	if snap.Op == store.VersionOpPurge {
		return nil, store.VersionMeta{}, store.ErrNotFound
	}
	e := entity.New(cur.ID, snap.Type)
	e.Face = snap.Face
	e.Content = snap.Content
	e.UpdatedAt = snap.CreatedAt
	maps.Copy(e.Properties, snap.Properties)
	return e, snap.VersionMeta, nil
}

// VersionTagReader resolves a version tag to the tagged version's snapshot
// (TKT-VO6VG9), in one consistent read. [store.VersionTagger] satisfies it;
// absent tags are [store.ErrNotFound].
type VersionTagReader interface {
	VersionByTag(ctx context.Context, ref entity.Ref, name store.VersionTagName) (*store.VersionSnapshot, error)
}

// WithVersionTags returns a copy of s that resolves version tags through t.
// Nil, or never called, leaves s without tags: [ScriptReader.VersionByTag]
// answers [store.ErrHistoryUnsupported].
func (s *ScriptReader) WithVersionTags(t VersionTagReader) *ScriptReader {
	c := *s
	c.tags = t
	return &c
}

// WithVersionTags is [ScriptReader.WithVersionTags] for the ungated reader.
func (r *UnrestrictedReader) WithVersionTags(t VersionTagReader) *UnrestrictedReader {
	c := *r
	c.tags = t
	return &c
}

// VersionByTag returns the entity face addr names as it was at the version
// tagged name, with that version's metadata. It is [ScriptReader.EntityVersion]
// at the tagged ordinal: the same gated read resolves the face, so a hidden
// entity is [store.ErrNotFound] exactly like a missing one, and the snapshot
// is redacted the same way. An absent tag, or a tag whose version cannot be
// served, is [store.ErrNotFound] too.
func (s *ScriptReader) VersionByTag(
	ctx context.Context, addr string, name store.VersionTagName,
) (*entity.Entity, store.VersionMeta, error) {
	ctx = s.bind(ctx)
	e, meta, err := versionByTag(ctx, s.history, s.tags, name, func() (*entity.Entity, error) {
		return s.res.addressAny(ctx, worldIn(ctx, s.world), addr)
	})
	if err != nil {
		return nil, store.VersionMeta{}, err
	}
	red, ok := s.reader.(rowRedactor)
	if !ok {
		return nil, store.VersionMeta{}, store.ErrNotFound
	}
	return red.RedactRow(affordances.WithHistoricalSubject(ctx), e), meta, nil
}

// VersionByTag is [ScriptReader.VersionByTag] without the gate or the
// redaction.
func (r *UnrestrictedReader) VersionByTag(
	ctx context.Context, addr string, name store.VersionTagName,
) (*entity.Entity, store.VersionMeta, error) {
	return versionByTag(ctx, r.history, r.tags, name, func() (*entity.Entity, error) {
		return r.GetAddress(ctx, addr)
	})
}

// VersionByTag denies, like every DenyReader read.
func (DenyReader) VersionByTag(context.Context, string, store.VersionTagName) (
	*entity.Entity, store.VersionMeta, error,
) {
	return nil, store.VersionMeta{}, store.ErrNotFound
}

// versionByTag resolves the live face once, looks the tag up in its lineage,
// and serves that version through [entityVersion]. The live read runs first,
// so the tag lookup never touches a face the gate refused.
func versionByTag(
	ctx context.Context, hr store.HistoryReader, tags VersionTagReader, name store.VersionTagName,
	live func() (*entity.Entity, error),
) (*entity.Entity, store.VersionMeta, error) {
	if hr == nil || tags == nil {
		return nil, store.VersionMeta{}, store.ErrHistoryUnsupported
	}
	cur, err := live()
	if err != nil {
		return nil, store.VersionMeta{}, err
	}
	snap, err := tags.VersionByTag(ctx, cur.Ref(), name)
	if err != nil {
		return nil, store.VersionMeta{}, err
	}
	return snapshotEntity(cur, snap)
}
