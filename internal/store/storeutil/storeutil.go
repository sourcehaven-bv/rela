// Package storeutil provides shared helpers for store.Store implementations.
//
// Functions here are used by both memstore and fsstore to avoid duplicating
// validation, filtering, and sorted-slice maintenance logic.
package storeutil

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// ValidateID is the store-boundary gate for entity IDs. It delegates to
// [entity.ValidateID], which owns the single ID grammar and documents why
// each rule exists.
//
// This function used to carry its own, looser rule (rejecting only empty,
// "--", path separators, and control characters). That was the validator
// actually enforced on every write path, while the stricter
// [entity.ValidateID] ran only on the manual-ID and rename paths — so
// generated IDs, the importer, and direct store writes could persist IDs
// containing spaces, shell metacharacters, or Unicode homoglyphs. The two
// have been collapsed into one rule (TKT-IZGF7T); do not reintroduce a
// store-local variant.
//
// It stays a named function rather than an alias for two reasons: it is the
// validity oracle for the storetest fuzz harness, and it prefixes errors with
// "store: " so a rejection surfaced from a backend names the layer that
// refused the write.
func ValidateID(id string) error {
	if err := entity.ValidateID(id); err != nil {
		return fmt.Errorf("store: %w", err)
	}
	return nil
}

// ValidateRelationType rejects relation types that would cause
// relation-key collisions or storage hazards. The rules mirror
// [ValidateID]: relation types are embedded in the same
// from--type--to key format (so `--` collides), become path segments
// in fsstore relation filenames (so separators nest directories), and
// appear in the pgstore change-feed payload whose field separator is
// a control character (internal/store/pgstore/feed.go already
// documents that assumption). Previously each backend hand-rolled a
// subset of these checks inline; the shared rule is also the validity
// oracle for the storetest fuzz harness (TKT-PCLGGL).
func ValidateRelationType(relType string) error {
	if relType == "" {
		return errors.New("store: empty relation type")
	}
	if strings.Contains(relType, "--") {
		return fmt.Errorf("store: relation type %q contains consecutive dashes", relType)
	}
	if strings.ContainsAny(relType, "/\\") {
		return fmt.Errorf("store: relation type %q contains path separator", relType)
	}
	for i := range len(relType) {
		if relType[i] < 0x20 || relType[i] == 0x7f {
			return fmt.Errorf("store: relation type %q contains control character", relType)
		}
	}
	return nil
}

// ValidateProperty rejects property names that would cause
// attachment key collisions in the entityID/property format.
func ValidateProperty(prop string) error {
	if prop == "" {
		return errors.New("store: empty property name")
	}
	if strings.Contains(prop, "/") {
		return fmt.Errorf("store: property name %q contains slash", prop)
	}
	return nil
}

// FoldID returns the case-folded form of an entity ID, for use as an
// identity key rather than as a display or storage value.
//
// Two IDs that fold to the same value are ONE entity (BUG-3RCWNS). This is
// not a stylistic rule — it is forced by fsstore, which writes "<id>.md" and
// so inherits the host filesystem's case behavior: on macOS and Windows
// "abc" and "ABC" are the same file. memstore and pgstore
// (id TEXT COLLATE "C") are byte-exact and would otherwise keep them as two
// separate entities.
//
// The backends must agree, because entities move between them: migrating a
// project that holds both "abc" and "ABC" into fsstore would silently drop
// one, and `rela sync` between a byte-exact and a case-folding store has no
// defined convergence. Enforcing identity in the shared layer is what keeps
// the backends substitutable (FEAT-CO4YP); the conformance suite pins it via
// CreateRejectsCaseVariantID.
//
// Uses strings.ToLower rather than strings.EqualFold's full Unicode folding
// because entity IDs are ASCII by construction (see entity.ValidateID), so
// the simple mapping is total over the legal input set and, unlike Unicode
// folding, cannot map two distinct ASCII IDs together.
//
// Callers must keep storing the ORIGINAL id — this value is only ever a
// lookup key. Casing is preserved in the entity and on disk.
func FoldID(id string) string {
	return strings.ToLower(id)
}

// SortedInsert adds key to a sorted slice, maintaining sort order.
func SortedInsert(s []string, key string) []string {
	i, _ := slices.BinarySearch(s, key)
	return slices.Insert(s, i, key)
}

// CompareStateKeys orders two entity state keys ("id" or "id@pointer")
// as the TUPLE (bare id, pointer), which is what pgstore's
// `ORDER BY id ASC, pointer ASC` does. One ordering contract across all
// three backends (TKT-WAV8XP).
//
// Plain string comparison on the joined key is NOT equivalent, and the
// difference is a correctness bug rather than a cosmetic one. The
// separator '@' is 0x40 and the digits are 0x30-0x39, so '@' sorts AFTER
// any digit and a numerically-prefixed sibling lands INSIDE the family:
//
//	string order: PAGE-1  PAGE-10  PAGE-10@draft  PAGE-1@draft  PAGE-2
//	tuple order:  PAGE-1  PAGE-1@draft  PAGE-10  PAGE-10@draft  PAGE-2
//
// World resolution buffers one family and decides at end-of-family, and
// the fallback verdict is a decision about ABSENCE — so a family split
// across a page boundary yields a WRONG prime, not a slow one (an entity
// dropped under `otherwise: exclude`, or its default face served when a
// published row exists further down). With `id_type: sequential` that
// starts at ten entities.
//
// A comparator rather than a lower-sorting separator on purpose: a
// separator encodes the invariant into an ASCII accident that a future
// id-grammar change would break silently.
func CompareStateKeys(a, b string) int {
	aID, aPtr, _ := strings.Cut(a, entity.StateRefSeparator)
	bID, bPtr, _ := strings.Cut(b, entity.StateRefSeparator)
	if c := strings.Compare(aID, bID); c != 0 {
		return c
	}
	return strings.Compare(aPtr, bPtr)
}

// SortedInsertFunc adds key to a slice sorted by cmp.
func SortedInsertFunc(s []string, key string, cmp func(a, b string) int) []string {
	i, _ := slices.BinarySearchFunc(s, key, cmp)
	return slices.Insert(s, i, key)
}

// SortedRemoveFunc removes key from a slice sorted by cmp. The key must
// exist — callers should only call this after confirming presence.
func SortedRemoveFunc(s []string, key string, cmp func(a, b string) int) []string {
	i, found := slices.BinarySearchFunc(s, key, cmp)
	if !found {
		panic("storeutil: SortedRemoveFunc called with missing key: " + key)
	}
	return slices.Delete(s, i, i+1)
}

// EncodeCursor turns a sort key into an opaque pagination cursor.
// Callers MUST NOT parse cursors — round-trip only via DecodeCursor.
func EncodeCursor(key string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(key))
}

// DecodeCursor recovers the sort key from a cursor produced by EncodeCursor.
// Returns an error for malformed input; an empty cursor decodes to "".
func DecodeCursor(cursor string) (string, error) {
	if cursor == "" {
		return "", nil
	}
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return "", fmt.Errorf("store: invalid cursor: %w", err)
	}
	return string(b), nil
}

// PageKeys holds the result of a paginated key scan: the sort keys that
// landed on this page (in order) and a cursor pointing past the last
// emitted key when more results exist.
type PageKeys struct {
	Keys       []string
	NextCursor string
}

// PaginateSortedKeys walks a pre-sorted slice of keys, selecting the
// next page of keys that satisfy match. It starts strictly after
// cursorKey — the key returned as the previous NextCursor is not
// re-emitted. When limit <= 0, every matching key is returned and
// NextCursor is "".
//
// NextCursor is set iff a matching key exists after the last emitted
// key, so an empty NextCursor is a reliable "no more results" signal.
// This costs one extra match call past the cut-off on the final page.
//
// Callers load the concrete items from the returned keys — keeping
// loads out of this helper lets each backend handle load errors in
// its own idiom (skip missing for in-memory, propagate I/O errors
// for fsstore).
func PaginateSortedKeys(
	sortedKeys []string,
	cursorKey string,
	limit int,
	match func(key string) bool,
) PageKeys {
	start := 0
	if cursorKey != "" {
		i, found := slices.BinarySearch(sortedKeys, cursorKey)
		start = i
		if found {
			start = i + 1
		}
	}

	page := PageKeys{}
	for i := start; i < len(sortedKeys); i++ {
		key := sortedKeys[i]
		if !match(key) {
			continue
		}
		if limit > 0 && len(page.Keys) == limit {
			// Already full and found another match — more results exist.
			page.NextCursor = EncodeCursor(page.Keys[len(page.Keys)-1])
			return page
		}
		page.Keys = append(page.Keys, key)
	}
	return page
}

// PaginateSortedKeysFunc is [PaginateSortedKeys] over a slice sorted by
// cmp rather than by plain string order (TKT-WAV8XP; see
// [CompareStateKeys]).
//
// A cursor issued before the ordering changed is a MISMATCHED cursor,
// which [store.EntityReader.ListEntitiesPage] documents as
// implementation-defined — cursors are opaque and valid only for the
// same query on the same store. It restarts from the beginning rather
// than resuming at a binary-search position derived under the old
// ordering, because that position would silently SKIP rows, which a
// paging caller cannot distinguish from end-of-results. Same reasoning
// as the unparseable-cursor restart (TKT-DOFYR1 PR-B).
func PaginateSortedKeysFunc(
	sortedKeys []string,
	cursorKey string,
	limit int,
	match func(key string) bool,
	cmp func(a, b string) int,
) PageKeys {
	start := 0
	if cursorKey != "" {
		i, found := slices.BinarySearchFunc(sortedKeys, cursorKey, cmp)
		start = i
		if found {
			start = i + 1
		}
	}

	page := PageKeys{}
	for i := start; i < len(sortedKeys); i++ {
		key := sortedKeys[i]
		if !match(key) {
			continue
		}
		if limit > 0 && len(page.Keys) == limit {
			page.NextCursor = EncodeCursor(page.Keys[len(page.Keys)-1])
			return page
		}
		page.Keys = append(page.Keys, key)
	}
	return page
}

// SortedRemove removes key from a sorted slice.
// The key must exist — callers should only call this after confirming presence.
func SortedRemove(s []string, key string) []string {
	i, found := slices.BinarySearch(s, key)
	if !found {
		panic("storeutil: SortedRemove called with missing key: " + key)
	}
	return slices.Delete(s, i, i+1)
}

// HeadlessStateError is the shared rejection for creating a non-default
// state with no default row (TKT-DOFYR1, design doc §6). One string
// across all backends so the contract cannot drift per backend.
func HeadlessStateError(id string) error {
	return fmt.Errorf("%w: entity %s has no default state; a state row cannot exist headless",
		store.ErrNotFound, id)
}

// StateTypeMismatchError is the shared rejection for a state whose type
// diverges from its family's (TKT-DOFYR1, design doc §6).
func StateTypeMismatchError(id string, p entity.Pointer, got, want string) error {
	return fmt.Errorf("state %s@%s type %q does not match the entity's type %q", id, p, got, want)
}

// MatchRelation returns true if a relation matches the given query.
//
// The tail-pointer filter is nil-permissive (TKT-DOFYR1): a nil
// q.FromPointer matches every tail — identity edges and all states —
// which is today's behavior for pointerless projects; non-nil matches
// by equality only (the store never inspects pointer contents).
func MatchRelation(r *entity.Relation, q store.RelationQuery) bool {
	if q.Type != "" && r.Type != q.Type {
		return false
	}
	if q.From != "" && r.From != q.From {
		return false
	}
	if q.FromPointer != nil && r.FromPointer != *q.FromPointer {
		return false
	}
	if q.To != "" && r.To != q.To {
		return false
	}
	if q.EntityID != "" {
		switch q.Direction {
		case store.DirectionOutgoing:
			if r.From != q.EntityID {
				return false
			}
		case store.DirectionIncoming:
			if r.To != q.EntityID {
				return false
			}
		default: // DirectionBoth
			if r.From != q.EntityID && r.To != q.EntityID {
				return false
			}
		}
	}
	return true
}

// ValidateEntityQuery rejects a query whose fields contradict each
// other. Today that is AllStates together with a non-default World
// (TKT-WAV8XP): AllStates is raw storage truth and world resolution is
// its opposite, so honoring both is impossible and honoring one
// silently is a precedence rule nobody remembers.
//
// It lives here rather than in each backend so every implementation
// inherits the same refusal — a backend that forgot the check would
// answer a contradictory query with a plausible-looking result, which
// is worse than an error. Pinned by the shared conformance suite.
func ValidateEntityQuery(q store.EntityQuery) error {
	if q.AllStates && !q.World.IsDefaultWorld() {
		return fmt.Errorf(
			"%w: AllStates and World are mutually exclusive — AllStates is raw storage "+
				"truth, a World resolves each entity to one state", store.ErrInvalidQuery)
	}
	return nil
}

// MatchEntityQuery reports whether an entity with the given type,
// bare id and pointer satisfies q's Type, IDs and AllStates filters.
// idSet must be pre-computed from q.IDs (see the backends' entityIDSet)
// and matches the BARE id, so IDs+AllStates selects every state of the
// listed entities.
//
// It takes the three fields rather than an *entity.Entity because
// fsstore matches against its in-memory index metadata, which
// deliberately holds no loaded entity — the previous byte-similar
// copies in fsstore and memstore had diverged in signature only.
//
// This is NOT world resolution and cannot be: a world picks at most one
// state per entity, which is a per-FAMILY ranked choice, so no
// per-row predicate can express it. World-scoped listing resolves
// primes separately, after matching.
//
// Under a non-default World the pointer filter is WIDENED rather than
// applied: every state of a family is a candidate, and resolution
// chooses among them afterwards. Keeping the default-only filter here
// would discard the very rows the chain selects, leaving a world able
// to return only default states — the failure would look like "the
// world does nothing" rather than an error.
func MatchEntityQuery(entityType, id string, p entity.Pointer, q store.EntityQuery, idSet map[string]bool) bool {
	if !q.AllStates && q.World.IsDefaultWorld() && !p.IsDefault() {
		return false
	}
	if q.Type != "" && entityType != q.Type {
		return false
	}
	if len(idSet) > 0 && !idSet[id] {
		return false
	}
	return true
}

// WorldCandidate is one stored state row offered to [WorldPrimes]: the
// bare id, its type, and its pointer. Backends pass their index metadata
// rather than a loaded entity, so resolution costs no I/O.
type WorldCandidate struct {
	ID      string
	Type    string
	Pointer entity.Pointer
}

// WorldPrimes selects, per bare id, the single state row that world w
// resolves to — the "prime" (TKT-WAV8XP, design doc §4.1). It returns
// the winning pointer per id; an id absent from the result contributes
// nothing to the world.
//
// Candidates must be the rows of WHOLE families: the fallback verdict is
// a decision about ABSENCE, so an id whose candidates are only partly
// present can resolve to the wrong prime rather than merely a stale one.
// That is why the fs/mem index sorts by [CompareStateKeys] and callers
// buffer a family before deciding.
//
// The three resolution rules:
//
//   - rule 1: the type has no resolution in w (absent from the scope) —
//     the DEFAULT state, matching the pre-worlds behavior. Absence is NOT
//     the zero TypeResolution, which excludes; see [store.WorldScope].
//   - rule 2: the first coordinate in the chain that EXISTS for this id.
//   - rule 3: no chain coordinate exists — the chain's Fallback decides,
//     either the default state or nothing at all.
//
// Decisions are made AFTER the whole candidate set is seen, never
// streaming per row: no backend documents an iteration order strong
// enough to conclude "this coordinate is absent" mid-family, and the
// same hazard already bit analysis.CheckStates.
func WorldPrimes(w store.WorldScope, candidates []WorldCandidate) map[string]entity.Pointer {
	if len(candidates) == 0 {
		return nil
	}

	// Per id: the best rank seen so far (lower wins), whether a default
	// row exists, and the id's type. Buffered, then decided below.
	type family struct {
		typ         string
		bestRank    int
		bestPtr     entity.Pointer
		haveBest    bool
		haveDefault bool
	}
	// The family's type comes from whichever row arrives first. Every
	// backend rejects a state whose type differs from its family's on
	// write (StateTypeMismatchError), so a family is single-typed and
	// the choice is arbitrary-but-equal. It is recorded rather than
	// re-derived because fsstore builds its index from directory
	// structure alone, without reading files: a hand-edited tree could
	// present a mixed-type family, and the answer should not then depend
	// on map iteration order upstream.
	fams := make(map[string]*family, len(candidates))
	for _, c := range candidates {
		f, ok := fams[c.ID]
		if !ok {
			f = &family{typ: c.Type}
			fams[c.ID] = f
		}
		if c.Pointer.IsDefault() {
			f.haveDefault = true
			// The default row is the family's identity row everywhere
			// else in the codebase; prefer its type so a (write-rejected,
			// hand-edited-tree-only) mixed-type family still resolves the
			// same way regardless of candidate order.
			f.typ = c.Type
		}
		res, scoped := w.For(c.Type)
		if !scoped {
			continue // rule 1: decided below from haveDefault
		}
		for rank, coord := range res.Chain {
			if coord != c.Pointer {
				continue
			}
			if !f.haveBest || rank < f.bestRank {
				f.bestRank, f.bestPtr, f.haveBest = rank, coord, true
			}
			break
		}
	}

	out := make(map[string]entity.Pointer, len(fams))
	for id, f := range fams {
		res, scoped := w.For(f.typ)
		switch {
		case !scoped:
			// Rule 1: the world says nothing about this type.
			if f.haveDefault {
				out[id] = entity.Pointer("")
			}
		case f.haveBest:
			// Rule 2.
			out[id] = f.bestPtr
		case res.Fallback == store.FallbackDefaultState && f.haveDefault:
			// Rule 3, `otherwise: default`.
			out[id] = entity.Pointer("")
		default:
			// Rule 3, `otherwise: exclude` — absence IS the publication
			// bit, so the id contributes nothing.
		}
	}
	return out
}

// PaginateWorldPrimes walks a contiguously-ordered key slice and returns
// up to limit PRIMES, plus the cursor to resume after the last family it
// emitted (TKT-WAV8XP).
//
// The limit counts primes, not candidate rows: an entity holding three
// faces consumes one slot of a page, not three. Memory is bounded by ONE
// FAMILY, not by the result set — which is the whole reason the index
// sorts by [CompareStateKeys]. Buffering everything and cutting
// afterwards would be correct but O(all matching rows) per page, and a
// page-at-a-time scan that decided mid-family would produce a WRONG
// prime, since the fallback verdict is a decision about absence.
//
// load maps a key to its candidate; a key that is absent (concurrently
// deleted) is skipped.
//
// match MUST be family-constant — it may only test properties shared by
// every row of a family (type, bare id), never a per-row property. Rows
// it rejects never reach [WorldPrimes], and resolution decides on
// ABSENCE: filtering away the published row makes the chain look
// unsatisfied, so the fallback fires and the default face is served in a
// world that meant to replace or exclude it. [MatchEntityQuery] is
// family-constant today (Type and IDs only). A per-row predicate pushed
// in here — a property filter, an updated-at bound, a pointer fast path
// — silently breaks world resolution in the serve-the-wrong-face
// direction; resolve the family first and filter the primes afterwards.
func PaginateWorldPrimes(
	sortedKeys []string,
	cursorKey string,
	limit int,
	w store.WorldScope,
	match func(key string) bool,
	load func(key string) (WorldCandidate, bool),
) PageKeys {
	start := 0
	if cursorKey != "" {
		i, found := slices.BinarySearchFunc(sortedKeys, cursorKey, CompareStateKeys)
		start = i
		if found {
			start = i + 1
		}
	}

	page := PageKeys{}
	var famKeys []string
	var famCands []WorldCandidate

	// flush resolves one buffered family and appends its prime, if any.
	// Returns false when the page is full and a further prime exists.
	flush := func() bool {
		if len(famCands) == 0 {
			return true
		}
		primes := WorldPrimes(w, famCands)
		defer func() { famKeys, famCands = famKeys[:0], famCands[:0] }()
		for i, c := range famCands {
			p, ok := primes[c.ID]
			if !ok || p != c.Pointer {
				continue
			}
			if limit > 0 && len(page.Keys) == limit {
				page.NextCursor = EncodeCursor(page.Keys[len(page.Keys)-1])
				return false
			}
			page.Keys = append(page.Keys, famKeys[i])
		}
		return true
	}

	curID := ""
	for i := start; i < len(sortedKeys); i++ {
		key := sortedKeys[i]
		c, ok := load(key)
		if !ok {
			continue
		}
		// A family boundary: everything about the previous id has been
		// seen, so it is safe to decide its prime.
		if c.ID != curID {
			if !flush() {
				return page
			}
			curID = c.ID
		}
		if !match(key) {
			continue
		}
		famKeys = append(famKeys, key)
		famCands = append(famCands, c)
	}
	flush()
	return page
}

// LimitAttachmentReader wraps r so reads fail with
// store.ErrAttachmentTooLarge once they exceed store.MaxAttachmentBytes.
// This is the shared backstop guard every store backend applies to
// AttachFile, so no backend is ever unbounded regardless of caller.
// Thin alias over store.CapAttachmentReader at the backstop cap.
func LimitAttachmentReader(r io.Reader) io.Reader {
	return store.CapAttachmentReader(r, store.MaxAttachmentBytes)
}
