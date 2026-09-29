package visibility

import (
	"context"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// ResolvedHeader is the answer for one address in [Resolver.ResolveHeaders].
type ResolvedHeader struct {
	// Header is the served face's redacted header. It is the zero header when
	// no face is served; see [ResolvedHeader.Served].
	Header store.EntityHeader
	// Family reports that the entity has some face the principal may read,
	// the [Resolver.Family] question. It is independent of the world. It
	// answers for the bare id only, never for a named face: for `ID@face`,
	// [ResolvedHeader.Served] is the face-level answer.
	Family bool
	served bool
}

// Served reports whether a face was served for the address.
func (h ResolvedHeader) Served() bool { return h.served }

// ResolveHeaders is the batch form of [Resolver.Address] and
// [Resolver.Family] for a caller that does not know the types. It answers
// every ref in refs from headers only, and is keyed by ref.
//
// Each ref runs the same gates as the single reads: the row gate on the bare
// id and the readable faces of its stored type admit it, the stored type
// comes from the header read (so a family stored under two types is a miss),
// and the served face must be readable. A named face serves that face. A bare
// id serves the face w resolves it to among the READABLE stored faces: the
// ACL trims the candidates first and the world ranks what is left, as
// [Resolver.InWorld] does. A denied world serves nothing, but Family still
// answers. A ref the address grammar refuses is a miss, as in
// [Resolver.Address].
//
// The cost does not depend on len(refs): one header query for every id, then
// one PermitsReadMany and one face-set lookup per stored type, then one
// traversal prime for the served rows.
//
// Every miss is absent from the result, whatever its cause. A failed header
// read and a gate error are logged and answered as misses: the gate runs
// only for ids the read found, so an error would tell an existing id from a
// missing one (see untyped.go).
func (r *Resolver) ResolveHeaders(
	ctx context.Context, w World, refs []entity.Ref,
) map[entity.Ref]ResolvedHeader {
	refs = wellFormed(refs)
	ids := refIDs(refs)
	if len(ids) == 0 {
		return nil
	}
	faces, ok := r.readableHeaders(ctx, ids)
	if !ok {
		return nil
	}
	return r.resolved(ctx, w, refs, faces)
}

// ReadableTypes answers, for each id in ids, the entity's stored type when
// the principal may read some face of it: the [Resolver.Family] question for
// a caller that holds only ids. An id absent from the result is unreadable or
// absent. It runs the same gates as [Resolver.ResolveHeaders] at the same
// cost, but serves, primes and redacts nothing.
//
// Unlike [Resolver.ResolveHeaders], a failed header read or a gate error is
// returned, and the result is nil. That is safe where the caller answers
// every fault as a failed request (a 500, not a per-id answer), which then
// says nothing about which id exists.
func (r *Resolver) ReadableTypes(ctx context.Context, ids []string) (map[string]string, error) {
	refs := make([]entity.Ref, len(ids))
	for i, id := range ids {
		refs[i] = entity.Ref{ID: id}
	}
	faces, err := r.scanHeaders(ctx, refIDs(refs), func(_ string, err error) error { return err })
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(faces))
	for id, hs := range faces {
		for _, h := range hs {
			out[id] = h.Type // scanHeaders keeps only single-type families
			break
		}
	}
	return out, nil
}

// resolved builds the per-ref answer from the readable headers.
func (r *Resolver) resolved(
	ctx context.Context, w World, refs []entity.Ref, faces map[string]map[entity.Face]store.EntityHeader,
) map[entity.Ref]ResolvedHeader {
	served := servedHeaders(w, refs, faces)
	probes := make([]*entity.Entity, 0, len(served))
	for _, h := range served {
		probes = append(probes, headerProbe(h))
	}
	ctx = PrimeTraversals(ctx, r.redact, probes)

	out := make(map[entity.Ref]ResolvedHeader, len(refs))
	for _, ref := range refs {
		readable := faces[ref.ID]
		if len(readable) == 0 {
			continue
		}
		res := ResolvedHeader{Family: true}
		if h, ok := served[ref]; ok {
			res.Header = RedactHeader(ctx, r.redact, h)
			res.served = true
		}
		out[ref] = res
	}
	return out
}

// wellFormed keeps the refs the address grammar accepts. A ref that does not
// survive a round trip through [entity.ParseRef] names no row.
func wellFormed(refs []entity.Ref) []entity.Ref {
	out := make([]entity.Ref, 0, len(refs))
	for _, ref := range refs {
		if parsed, err := entity.ParseRef(ref.String()); err == nil && parsed == ref {
			out = append(out, ref)
		}
	}
	return out
}

// refIDs returns the distinct non-empty ids of refs, in first-seen order.
func refIDs(refs []entity.Ref) []string {
	seen := make(map[string]bool, len(refs))
	var ids []string
	for _, ref := range refs {
		if ref.ID != "" && !seen[ref.ID] {
			seen[ref.ID] = true
			ids = append(ids, ref.ID)
		}
	}
	return ids
}

// readableHeaders reads every stored face header of ids in one query and
// keeps, per id, the headers of the faces the principal may read. An id
// missing from the result has no readable face. ok is false when the read
// failed.
//
// A gate error is logged and hides every id of its type.
func (r *Resolver) readableHeaders(
	ctx context.Context, ids []string,
) (map[string]map[entity.Face]store.EntityHeader, bool) {
	out, err := r.scanHeaders(ctx, ids, func(typ string, err error) error {
		warnGate("batch", typ, entity.Ref{}, err)
		return nil
	})
	if err != nil {
		slog.Warn("visibility: header read failed; answering not-found",
			"ids", len(ids), "err", err)
		return nil, false
	}
	return out, true
}

// scanHeaders is the shared core of the batch reads. It returns a failed
// header read as an error. A gate error for one type goes to onGateErr: a nil
// return hides that type's ids and carries on, a non-nil one aborts the scan
// with that error.
func (r *Resolver) scanHeaders(
	ctx context.Context, ids []string, onGateErr func(typ string, err error) error,
) (map[string]map[entity.Face]store.EntityHeader, error) {
	if len(ids) == 0 {
		return map[string]map[entity.Face]store.EntityHeader{}, nil
	}
	stored, err := r.storedHeaders(ctx, store.EntityQuery{IDs: ids, AllStates: true})
	if err != nil {
		return nil, err
	}
	return r.gateHeaders(ctx, stored, onGateErr)
}

// storedHeaders reads the headers q selects, grouped by id.
func (r *Resolver) storedHeaders(ctx context.Context, q store.EntityQuery) (map[string][]store.EntityHeader, error) {
	stored := make(map[string][]store.EntityHeader, len(q.IDs))
	for h, err := range store.ListEntityHeaders(ctx, r.load, q) {
		if err != nil {
			return nil, err
		}
		stored[h.ID] = append(stored[h.ID], h)
	}
	return stored, nil
}

// gateHeaders keeps, per id, the headers of the faces the principal may
// read: one PermitsReadMany and one face-set lookup per stored type. A
// family stored under two types is dropped. onGateErr is as for
// scanHeaders.
func (r *Resolver) gateHeaders(
	ctx context.Context, stored map[string][]store.EntityHeader, onGateErr func(typ string, err error) error,
) (map[string]map[entity.Face]store.EntityHeader, error) {
	byType := make(map[string][]string)
	for id, hs := range stored {
		if typ, ok := singleType(hs); ok {
			byType[typ] = append(byType[typ], id)
		}
	}
	out := make(map[string]map[entity.Face]store.EntityHeader, len(stored))
	for typ, typeIDs := range byType {
		perm, faces, err := r.typeGate(ctx, typ, typeIDs)
		if err != nil {
			if abort := onGateErr(typ, err); abort != nil {
				return nil, abort
			}
			continue
		}
		for _, id := range typeIDs {
			if !perm[id] {
				continue
			}
			for _, h := range stored[id] {
				if !faces.Contains(h.Face) {
					continue
				}
				if out[id] == nil {
					out[id] = make(map[entity.Face]store.EntityHeader)
				}
				out[id][h.Face] = h
			}
		}
	}
	return out, nil
}

// singleType returns the type every header in hs is stored under. A family
// stored under two types is corrupt, so neither claim is trusted.
func singleType(hs []store.EntityHeader) (string, bool) {
	typ := hs[0].Type
	for _, h := range hs[1:] {
		if h.Type != typ {
			return "", false
		}
	}
	return typ, true
}

// typeGate runs the row gate over ids and reads the readable faces of typ.
func (r *Resolver) typeGate(ctx context.Context, typ string, ids []string) (map[string]bool, FaceSet, error) {
	perm, err := r.gate.PermitsReadMany(ctx, typ, ids)
	if err != nil {
		return nil, FaceSet{}, err
	}
	faces, err := ReadableFaces(ctx, r.gate, typ)
	if err != nil {
		return nil, FaceSet{}, err
	}
	return perm, faces, nil
}

// servedHeaders picks, per ref, the header of the face it serves. A named
// face serves itself when readable; a bare id serves the face w resolves it
// to among its readable faces.
func servedHeaders(
	w World, refs []entity.Ref, faces map[string]map[entity.Face]store.EntityHeader,
) map[entity.Ref]store.EntityHeader {
	out := make(map[entity.Ref]store.EntityHeader, len(refs))
	if w.denied {
		return out
	}
	var candidates []store.WorldCandidate
	offered := make(map[string]bool)
	for _, ref := range refs {
		if !ref.Face.IsDefault() {
			if h, ok := faces[ref.ID][ref.Face]; ok {
				out[ref] = h
			}
			continue
		}
		if offered[ref.ID] {
			continue
		}
		offered[ref.ID] = true
		for _, h := range faces[ref.ID] {
			candidates = append(candidates, store.WorldCandidate{ID: h.ID, Type: h.Type, Face: h.Face})
		}
	}
	primes := store.ResolveWorldPrimes(w.scope, candidates)
	for _, ref := range refs {
		if !ref.Face.IsDefault() {
			continue
		}
		if p, ok := primes[ref.ID]; ok {
			out[ref] = faces[ref.ID][p.Face]
		}
	}
	return out
}
