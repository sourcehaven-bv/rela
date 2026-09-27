package worldreader

import (
	"context"
	"errors"
	"iter"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Source resolves the world for the operation on ctx. The zero
// [store.WorldScope] is the default world. A [BoundReader] calls it once per
// read, so a source may read hot-reloaded configuration and the ctx
// principal's grants.
//
// An error is an infrastructure failure: it is returned to the caller rather
// than rendered as an empty result.
type Source func(ctx context.Context) (store.WorldScope, error)

// EntityReader is the read surface a [BoundReader] wraps and serves. In
// production it is the ACL-gated reader, so every row the world ranks has
// already passed the row gate and the face gate.
type EntityReader interface {
	GetEntity(ctx context.Context, id string) (*entity.Entity, error)
	ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error]
}

// BoundReader resolves entity reads through a world for callers that address
// entities by id and never name a world themselves: MCP tools and Lua
// scripts. Without it they read the default world, where a faced type has no
// row at all (BUG-6XTX0G).
//
// Addressing follows the data-entry read API:
//
//   - a bare id resolves to the entity's prime in the bound world;
//   - an explicit `ID@face` is served literally in every world, because the
//     caller named the row and there is nothing left for a world to decide;
//   - a list query that names no world and no AllStates gets the bound world
//     stamped on it. A query that names a world or AllStates is passed
//     through. A FaceIn set narrows the candidates the world ranks, but the
//     ACL pushdown in the production reader replaces it with the grant's own
//     face set, so do not rely on it there.
type BoundReader struct {
	inner  EntityReader
	source Source
}

// NewBoundReader wraps inner so its reads resolve through the world that
// source returns.
//
// Nil: inner and source are rejected — a reader with no world would silently
// read the default world, which is the defect this type exists to remove.
func NewBoundReader(inner EntityReader, source Source) (*BoundReader, error) {
	if inner == nil {
		return nil, errors.New("worldreader.NewBoundReader: inner reader is required")
	}
	if source == nil {
		return nil, errors.New("worldreader.NewBoundReader: world source is required")
	}
	return &BoundReader{inner: inner, source: source}, nil
}

// GetEntity returns the entity ref addresses in the bound world.
//
// A bare id in a non-default world is resolved over the faces the inner
// reader returns, which are the faces the caller may read. Filtering before
// ranking matches the data-entry single-entity GET: a reader granted only
// `policy@concept` under `select: [adopted, concept]` gets the concept face
// rather than a not-found.
func (b *BoundReader) GetEntity(ctx context.Context, ref string) (*entity.Entity, error) {
	scope, err := b.source(ctx)
	if err != nil {
		return nil, err
	}
	id, face, err := entity.ParseStateRef(ref)
	if err != nil {
		// Not an address this package can resolve. The inner reader answers
		// exactly as it did before worlds, which for an invalid id is a miss.
		return b.inner.GetEntity(ctx, ref)
	}
	if !face.IsDefault() {
		// AllStates, because outside a world a store matches default rows
		// only, and FaceIn would then narrow those to nothing.
		return b.first(ctx, store.EntityQuery{IDs: []string{id}, FaceIn: []entity.Face{face}, AllStates: true})
	}
	if scope.IsDefaultWorld() {
		return b.inner.GetEntity(ctx, id)
	}
	return b.prime(ctx, id, scope)
}

// ListEntities yields the entities q matches in the bound world.
//
// A typed query reaches the ACL pushdown with its world, where the face
// allowlist filters before the world ranks. A type-less query is resolved by
// the store and then filtered, so a face-restricted reader can miss an entity
// whose prime is a face they may not read. That only ever narrows the result.
func (b *BoundReader) ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	return func(yield func(*entity.Entity, error) bool) {
		scope, err := b.source(ctx)
		if err != nil {
			yield(nil, err)
			return
		}
		if q.World.IsDefaultWorld() && !q.AllStates {
			q.World = scope
		}
		for e, err := range b.inner.ListEntities(ctx, q) {
			if !yield(e, err) || err != nil {
				return
			}
		}
	}
}

// prime reads every face of id the inner reader returns and serves the one
// scope resolves to. The rows are one whole family as the caller sees it,
// which is the candidate set [store.ResolveWorldPrimes] requires.
func (b *BoundReader) prime(ctx context.Context, id string, scope store.WorldScope) (*entity.Entity, error) {
	var rows []*entity.Entity
	for e, err := range b.inner.ListEntities(ctx, store.EntityQuery{IDs: []string{id}, AllStates: true}) {
		if err != nil {
			return nil, err
		}
		if e != nil && e.ID == id {
			rows = append(rows, e)
		}
	}
	candidates := make([]store.WorldCandidate, len(rows))
	for i, e := range rows {
		candidates[i] = store.WorldCandidate{ID: e.ID, Type: e.Type, Face: e.Face}
	}
	resolved, ok := store.ResolveWorldPrimes(scope, candidates)[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	for _, e := range rows {
		if e.Face == resolved.Face {
			return e, nil
		}
	}
	return nil, store.ErrNotFound
}

// first returns the first row q yields, or [store.ErrNotFound].
func (b *BoundReader) first(ctx context.Context, q store.EntityQuery) (*entity.Entity, error) {
	for e, err := range b.inner.ListEntities(ctx, q) {
		if err != nil {
			return nil, err
		}
		// The id and face checks guard against a reader that replaces the
		// query's IDs or FaceIn, as the ACL pushdown does for a typed query.
		if e != nil && slices.Contains(q.IDs, e.ID) && (len(q.FaceIn) == 0 || slices.Contains(q.FaceIn, e.Face)) {
			return e, nil
		}
	}
	return nil, store.ErrNotFound
}
