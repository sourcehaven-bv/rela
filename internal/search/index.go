// Package search provides full-text search over a [store.Store],
// satisfied by a [Backend] (today: [bleveindex.Index]). The [Service]
// type combines a [store.EntityReader] with a [Backend] to produce a
// [Searcher] implementation that callers consume.
//
// # A world IS the search scope
//
// Worlds (TKT-WAV8XP) reach this package by exactly one route: [Query.World]
// carries a [store.WorldScope], and [Backend.Search] takes it. Nothing else
// here knows what a world is — no world NAME, no metamodel, no policy. The
// scope arrives compiled, from the consumer that resolved it.
//
// The scope RESOLVES rather than filters, and that is the load-bearing
// choice ([ResolvePrimes] carries the argument in full). A world picks at
// most one face per entity — its prime — and only that face is matched. So:
//
//   - A hit's text is the text the reader will be shown. Matching a
//     non-prime face would let a published-world search hit on a term that
//     exists only in the draft while displaying published bytes that lack
//     it.
//   - An entity the world resolves to nothing contributes nothing. Under
//     `otherwise: exclude` that absence IS the publication bit.
//   - At most one row per entity comes out STRUCTURALLY, so [Query.Limit]
//     counts entities with no grouping pass.
//
// The zero [store.WorldScope] is the default world and must reduce to
// exactly the pre-worlds query — every construction site that never heard
// of worlds keeps working, allocating nothing.
//
// Consequently the ACL row gate is NOT what keeps a draft out of a
// published-world result: guard rule 1 makes that gate world-independent.
// The resolution above is. A backend that matched first and resolved after
// (or skipped resolution entirely) would leak drafts with nothing
// downstream to catch it — see the doc on [Backend.Search].
package search

import (
	"cmp"
	"context"
	"fmt"
	"iter"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Service implements Searcher by combining an EntityReader with a Backend.
// Text queries go to the backend; type/property filters are applied by
// loading entities from the reader.
type Service struct {
	reader  store.EntityReader
	backend Backend
}

// compile-time check
var _ Searcher = (*Service)(nil)

// New creates a Searcher backed by the given reader and search backend.
func New(reader store.EntityReader, backend Backend) *Service {
	return &Service{reader: reader, backend: backend}
}

// MatchedFields reports which logical fields of the already-loaded entity the
// query text matched, in the [FieldID] / [FieldContent] / [PropFieldPrefix]
// vocabulary. It delegates to the backend's [FieldMatcher] when the backend
// implements one, so provenance is computed with the same matcher the backend
// searched with; otherwise it falls back to the ground-truth [MatchTextFields].
//
// It takes the entity rather than loading it so the seam can thread a single
// snapshot through both provenance and the hidden-field verdict (no
// re-load, no cross-snapshot race). An empty text query has no provenance and
// returns nil.
func (s *Service) MatchedFields(e *entity.Entity, text string) map[string]struct{} {
	if e == nil || text == "" {
		return nil
	}
	if fm, ok := s.backend.(FieldMatcher); ok {
		return fm.MatchedFields(e, text)
	}
	return MatchTextFields(e, text)
}

func (s *Service) Search(ctx context.Context, q Query) iter.Seq2[Hit, error] {
	if err := ValidateQuery(q); err != nil {
		return func(yield func(Hit, error) bool) {
			yield(Hit{}, err)
		}
	}

	if q.Text == "" {
		return s.listAll(ctx, q)
	}

	// The backend resolves the world and returns FACES, so each result
	// already names which face matched. The limit is applied here rather
	// than pushed down because the type and property filters below can still
	// reject a face, and a backend-side limit would silently shorten the
	// page.
	faces, err := s.backendSearch(q)
	if err != nil {
		return func(yield func(Hit, error) bool) {
			yield(Hit{}, err)
		}
	}

	typeSet := toSet(q.Types)

	return func(yield func(Hit, error) bool) {
		emitted := 0
		for page := range slices.Chunk(faces, searchLoadPage) {
			rows, err := s.loadFaces(ctx, page)
			if err != nil {
				yield(Hit{}, err)
				return
			}
			for _, f := range page {
				if q.Limit > 0 && emitted >= q.Limit {
					return
				}
				hit, ok := pageHit(rows, f, typeSet, q.Filters)
				if !ok {
					continue
				}
				if !yield(hit, nil) {
					return
				}
				emitted++
			}
		}
	}
}

// pageHit builds the hit for backend face f from the page's loaded rows. It
// reports false when the face is gone (deleted since indexing) or fails the
// type or property filters.
func pageHit(
	rows map[entity.Ref]*entity.Entity, f Face, typeSet map[string]bool, filters []PropertyFilter,
) (Hit, bool) {
	e, ok := rows[entity.Ref{ID: f.ID, Face: f.Face}]
	if !ok {
		return Hit{}, false
	}
	if len(typeSet) > 0 && !typeSet[e.Type] {
		return Hit{}, false
	}
	if !MatchFilters(e, filters) {
		return Hit{}, false
	}
	return Hit{
		ID:            e.ID,
		Type:          e.Type,
		Title:         e.Title(),
		Face:          f.Face,
		Via:           f.Via,
		ChainPosition: f.ChainPosition,
	}, true
}

// searchLoadPage is how many backend hits [Service.Search] reads with one
// store call.
const searchLoadPage = 256

// loadFaces reads the headers of the face rows that page names, in one store
// call, keyed by ref. A hit needs only its type, title and properties, so no
// body is read (collection-reads rule).
//
// It reads the FACE that matched, not the entity's default state: the bare id
// would show default-face values for a hit scored against a different face,
// the mismatch between what was searched and what is shown that world-scoped
// search exists to close. A face deleted since indexing is absent.
func (s *Service) loadFaces(ctx context.Context, page []Face) (map[entity.Ref]*entity.Entity, error) {
	ids := make([]string, 0, len(page))
	want := make(map[entity.Ref]bool, len(page))
	for _, f := range page {
		want[entity.Ref{ID: f.ID, Face: f.Face}] = true
		ids = append(ids, f.ID)
	}
	slices.Sort(ids)
	ids = slices.Compact(ids)
	rows := make(map[entity.Ref]*entity.Entity, len(want))
	q := store.EntityQuery{IDs: ids, Faces: store.AllFaces()}
	for h, err := range store.ListEntityHeaders(ctx, s.reader, q) {
		if err != nil {
			return nil, fmt.Errorf("search: load hits: %w", err)
		}
		ref := entity.Ref{ID: h.ID, Face: h.Face}
		if want[ref] {
			rows[ref] = &entity.Entity{ID: h.ID, Type: h.Type, Face: h.Face, Properties: h.Properties}
		}
	}
	return rows, nil
}

// backendSearch runs the text query on the backend, with q.Admit applied
// before the world ranks. A backend that cannot admit refuses an admitting
// query: ranking unadmitted faces would pick a prime the reader may not see
// and hide the entity instead of serving its readable face.
func (s *Service) backendSearch(q Query) ([]Face, error) {
	if q.Admit == nil {
		return s.backend.Search(q.Text, 0, q.World)
	}
	ab, ok := s.backend.(AdmittingBackend)
	if !ok {
		return nil, fmt.Errorf("%w: search backend %T cannot admit faces before ranking", ErrScope, s.backend)
	}
	return ab.SearchAdmitted(q.Text, 0, q.World, q.Admit)
}

// listAll handles searches with no text query: the rows of q.Types (every
// type when empty) in id order that pass q.Filters, at most q.Limit of them.
//
// It reads the store a page at a time (Service.pagedRows) and stops once
// the limit is met, so a limited search reads about a page, not the store
// (TKT-D9UO9Y4). A filter that rejects most rows still reads until it has
// found the limit; filters are not pushed into the store.
func (s *Service) listAll(ctx context.Context, q Query) iter.Seq2[Hit, error] {
	if q.Admit != nil {
		return s.listAdmitted(ctx, q)
	}
	return func(yield func(Hit, error) bool) {
		emitted := 0
		for e, err := range s.pagedRows(ctx, q.Types, store.InWorld(q.World), listPageSize(q)) {
			if err != nil {
				yield(Hit{}, err)
				return
			}
			if !MatchFilters(e, q.Filters) {
				continue
			}
			if !yield(Hit{ID: e.ID, Type: e.Type, Title: e.Title(), Face: e.Face}, nil) {
				return
			}
			emitted++
			if q.Limit > 0 && emitted >= q.Limit {
				return
			}
		}
	}
}

// firstListPage is the first page Service.pagedRows reads when the limit
// does not size it. Each later page doubles, up to [searchLoadPage], so a
// search that stops early reads little and a long scan costs few calls.
const firstListPage = 32

// listPageSize is the first page Service.listAll and Service.listAdmitted
// read: the limit when every row read becomes a hit, so a limited search
// reads one page, and [firstListPage] otherwise.
func listPageSize(q Query) int {
	if q.Limit > 0 && q.Limit < searchLoadPage && len(q.Filters) == 0 && q.Admit == nil {
		return q.Limit
	}
	return firstListPage
}

// listAdmitted is listAll with q.Admit applied before the world ranks. It
// reads every face in id order, a page at a time, and admits and resolves
// whole families a batch at a time. Once a batch holds as many rows as the
// store page, every family in it but the last is admitted; the last may
// continue in the next page, so its rows wait for the next batch. No family
// is split across two admissions, and no page is read before the rows
// already read are used. Batches grow as Service.typePages pages do, so a
// long scan admits in few calls.
func (s *Service) listAdmitted(ctx context.Context, q Query) iter.Seq2[Hit, error] {
	return func(yield func(Hit, error) bool) {
		size := listPageSize(q)
		batchSize := size
		out := &batchEmitter{q: q, yield: yield}
		flush := out.flush
		var batch []*entity.Entity
		for e, err := range s.pagedRows(ctx, q.Types, store.AllFaces(), size) {
			if err != nil {
				yield(Hit{}, err)
				return
			}
			batch = append(batch, e)
			if len(batch) < batchSize {
				continue
			}
			cut := lastFamilyStart(batch)
			if cut == 0 {
				continue // one family fills the batch; it may go on
			}
			if !flush(batch[:cut]) {
				return
			}
			batch = slices.Clone(batch[cut:])
			batchSize = nextPageSize(batchSize)
		}
		if len(batch) > 0 {
			flush(batch)
		}
	}
}

// batchEmitter yields the hits of admitted batches until q.Limit of them
// were yielded.
type batchEmitter struct {
	q       Query
	yield   func(Hit, error) bool
	emitted int
}

// flush admits rows and yields their hits; it reports whether to read on.
func (b *batchEmitter) flush(rows []*entity.Entity) bool {
	hits, err := admitBatch(b.q, rows)
	if err != nil {
		b.yield(Hit{}, err)
		return false
	}
	for _, h := range hits {
		if !b.yield(h, nil) {
			return false
		}
		b.emitted++
		if b.q.Limit > 0 && b.emitted >= b.q.Limit {
			return false
		}
	}
	return true
}

// lastFamilyStart is the index of the first row of the last id in rows,
// which are in id order.
func lastFamilyStart(rows []*entity.Entity) int {
	last := rows[len(rows)-1].ID
	return slices.IndexFunc(rows, func(r *entity.Entity) bool { return r.ID == last })
}

// admitBatch admits the faces of rows, whole families in id order, resolves
// each id's prime in q.World, and returns the hits that pass q.Filters, in
// id order.
func admitBatch(q Query, rows []*entity.Entity) ([]Hit, error) {
	byCand := make(map[Candidate]*entity.Entity, len(rows))
	cands := make([]Candidate, 0, len(rows))
	var order []string
	for _, e := range rows {
		c := Candidate{ID: e.ID, Type: e.Type, Face: e.Face}
		if _, seen := byCand[c]; seen {
			continue
		}
		if len(order) == 0 || order[len(order)-1] != e.ID {
			order = append(order, e.ID)
		}
		byCand[c] = e
		cands = append(cands, c)
	}
	admitted, err := q.Admit(cands)
	if err != nil {
		return nil, err
	}
	typeOf := make(map[string]string, len(admitted))
	for _, c := range admitted {
		typeOf[c.ID] = c.Type
	}
	primes := ResolvePrimes(q.World, admitted)
	hits := make([]Hit, 0, len(primes))
	for _, id := range order {
		res, ok := primes[id]
		if !ok {
			continue
		}
		e := byCand[Candidate{ID: id, Type: typeOf[id], Face: res.Face}]
		if e == nil || !MatchFilters(e, q.Filters) {
			continue
		}
		hits = append(hits, Hit{ID: e.ID, Type: e.Type, Title: e.Title(), Face: e.Face, Via: res.Via,
			ChainPosition: res.ChainPosition})
	}
	return hits, nil
}

// pagedRows yields the rows of types (every type when empty) at faces, in
// id order and then face order, reading size rows in the first store call
// (Service.typePages). Each type
// is its own keyset-paged read; several types are merged by key, so the
// order does not depend on how many types are asked for.
func (s *Service) pagedRows(
	ctx context.Context, types []string, faces store.FaceSelection, size int,
) iter.Seq2[*entity.Entity, error] {
	types = slices.Compact(slices.Sorted(slices.Values(types)))
	if len(types) == 0 {
		return s.typePages(ctx, "", faces, size)
	}
	streams := make([]iter.Seq2[*entity.Entity, error], len(types))
	for i, t := range types {
		streams[i] = s.typePages(ctx, t, faces, size)
	}
	if len(streams) == 1 {
		return streams[0]
	}
	return mergeRows(streams)
}

// typePages yields the rows of one type (every type when typ is empty) a
// keyset page at a time: size rows first, then twice as many each page, up
// to [searchLoadPage].
func (s *Service) typePages(
	ctx context.Context, typ string, faces store.FaceSelection, size int,
) iter.Seq2[*entity.Entity, error] {
	return func(yield func(*entity.Entity, error) bool) {
		q := store.EntityQuery{Type: typ, Faces: faces, Limit: size}
		for {
			page, err := s.reader.ListEntitiesPage(ctx, q)
			if err != nil {
				yield(nil, fmt.Errorf("search: list: %w", err))
				return
			}
			for _, e := range page.Items {
				if !yield(e, nil) {
					return
				}
			}
			if page.NextCursor == "" {
				return
			}
			q.Cursor = page.NextCursor
			q.Limit = nextPageSize(q.Limit)
		}
	}
}

// nextPageSize is the page read after one of size rows: twice as large, up
// to [searchLoadPage], and never smaller.
func nextPageSize(size int) int {
	return max(size, min(2*size, searchLoadPage))
}

// mergeHead is one stream of [mergeRows] and the row it is at; a nil row
// means the stream is done.
type mergeHead struct {
	next func() (*entity.Entity, error, bool)
	row  *entity.Entity
}

// advance moves h to its stream's next row.
func (h *mergeHead) advance() error {
	e, err, ok := h.next()
	h.row = nil
	if err != nil {
		return err
	}
	if ok {
		h.row = e
	}
	return nil
}

// leastHead is the head at the least row, or nil when every stream is done.
func leastHead(heads []*mergeHead) *mergeHead {
	var least *mergeHead
	for _, h := range heads {
		if h.row != nil && (least == nil || compareRowKey(h.row, least.row) < 0) {
			least = h
		}
	}
	return least
}

// mergeRows merges streams that are each in (id, face) order into one
// stream in that order.
func mergeRows(streams []iter.Seq2[*entity.Entity, error]) iter.Seq2[*entity.Entity, error] {
	return func(yield func(*entity.Entity, error) bool) {
		heads := make([]*mergeHead, 0, len(streams))
		stops := make([]func(), 0, len(streams))
		defer func() {
			for _, stop := range stops {
				stop()
			}
		}()
		for _, st := range streams {
			next, stop := iter.Pull2(st)
			stops = append(stops, stop)
			h := &mergeHead{next: next}
			heads = append(heads, h)
			if err := h.advance(); err != nil {
				yield(nil, err)
				return
			}
		}
		for least := leastHead(heads); least != nil; least = leastHead(heads) {
			if !yield(least.row, nil) {
				return
			}
			if err := least.advance(); err != nil {
				yield(nil, err)
				return
			}
		}
	}
}

// compareRowKey orders rows by id, then face: the store's list order.
func compareRowKey(a, b *entity.Entity) int {
	return cmp.Or(cmp.Compare(a.ID, b.ID), cmp.Compare(a.Face, b.Face))
}

func toSet(ss []string) map[string]bool {
	if len(ss) == 0 {
		return nil
	}
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}
