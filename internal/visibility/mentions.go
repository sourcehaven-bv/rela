package visibility

import (
	"context"
	"errors"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// ErrMentionsUnsupported is returned by [Resolver.MentionedBy] when the
// resolver's loader has no [store.MentionIndex].
var ErrMentionsUnsupported = errors.New("visibility: the store keeps no mention index")

// mentionScanBudget bounds the raw index rows one [Resolver.MentionedBy] call
// reads, so a target mentioned by many rows the principal cannot read costs
// at most this many per call. A cursor may only name a served row, so a call
// that runs out of budget cannot hand back where it stopped: sources behind
// a longer run of unreadable ones are reported as Truncated, not reached.
// The budget is set high so that only an extreme project hits it.
const mentionScanBudget = 10000

// mentionFetch is the raw page size [Resolver.MentionedBy] reads at a time.
const mentionFetch = 500

// MentionPage is one page of [Resolver.MentionedBy].
//
// Nothing in it is derived from a row the principal cannot read: Next is
// the id of the last item served, never a raw index row, so a cursor cannot
// name a hidden entity or a hidden face.
type MentionPage struct {
	// Items are the served, redacted headers of the mentioning entities, in
	// id order. An entity appears once, at the face w serves.
	Items []store.EntityHeader
	// Next is the cursor for the following page: the id of the last item.
	// It is "" on the last page, or when the scan stopped short.
	Next string
	// Truncated reports that the scan stopped at its budget, so more
	// sources may exist that no cursor reaches. It reveals only that many
	// unreadable rows follow, not which; docs/acl-security.md records that
	// as accepted.
	Truncated bool
}

// ErrBadMentionCursor is returned by [ParseMentionCursor] for a string no
// [MentionPage] produced.
var ErrBadMentionCursor = errors.New("visibility: not a mention cursor")

// ParseMentionCursor checks a cursor a caller passes back: "" or an entity
// id. Any id is safe to accept, since paging from it serves only rows the
// principal may read.
func ParseMentionCursor(cursor string) (string, error) {
	if cursor == "" {
		return "", nil
	}
	if entity.ValidateID(cursor) != nil {
		return "", ErrBadMentionCursor
	}
	return cursor, nil
}

// MentionedBy pages the entities whose body mentions target (TKT-Q6AKUN),
// starting after the id after ("" starts at the beginning).
//
// Each mentioning id goes through [Resolver.ResolveIDsErr], so the row gate,
// the face gate, the world and redaction apply exactly as for a list read. An
// id is listed only if the face w serves for it is itself a face that
// mentions target: otherwise a reader would see a backlink from a body that,
// as served to them, says nothing about target. An entity that mentions
// itself is not its own backlink.
//
// It is the only sanctioned reader of [store.MentionIndex]; an archguard
// test keeps every other package from calling the index directly.
func (r *Resolver) MentionedBy(
	ctx context.Context, w World, target, after string, limit int,
) (MentionPage, error) {
	if limit <= 0 {
		return MentionPage{}, store.ErrInvalidQuery
	}
	idx, ok := r.load.(store.MentionIndex)
	if !ok {
		return MentionPage{}, ErrMentionsUnsupported
	}
	// The raw position stays internal. Starting at the implicit face of
	// after also returns after's other faces; the id filter drops them.
	var cursor entity.Ref
	if after != "" {
		parsed, err := entity.ParseRef(after)
		if err != nil {
			return MentionPage{}, ErrBadMentionCursor
		}
		cursor = parsed
	}
	var page MentionPage
	for scanned := 0; ; {
		refs, full, err := mentionBatch(ctx, idx, target, cursor)
		if err != nil {
			return MentionPage{}, err
		}
		if len(refs) == 0 {
			return page, nil
		}
		scanned += len(refs)
		cursor = refs[len(refs)-1]
		ids, faces := groupRefs(refs)
		ids = slices.DeleteFunc(ids, func(id string) bool { return id <= after || id == target })
		served, err := r.ResolveIDsErr(ctx, w, ids)
		if err != nil {
			return MentionPage{}, err
		}
		for _, id := range ids {
			h, ok := served[id]
			if !ok || !slices.Contains(faces[id], h.Face) {
				continue
			}
			// One item past the page tells a full last page from a page
			// with more behind it; it is dropped, not served.
			if len(page.Items) == limit {
				page.Next = page.Items[limit-1].ID
				return page, nil
			}
			page.Items = append(page.Items, h)
		}
		if !full {
			return page, nil
		}
		if scanned >= mentionScanBudget {
			page.Truncated = true
			return page, nil
		}
	}
}

// mentionBatch reads the next raw refs after cursor, ending on a whole id: a
// family's faces are adjacent in ref order, and judging an id needs all of
// its mentioning faces. full reports whether more refs may follow.
func mentionBatch(
	ctx context.Context, idx store.MentionIndex, target string, cursor entity.Ref,
) (refs []entity.Ref, full bool, err error) {
	for {
		batch, err := idx.MentionSources(ctx, target, cursor, mentionFetch)
		if err != nil {
			return nil, false, err
		}
		refs = append(refs, batch...)
		if len(batch) < mentionFetch {
			return refs, false, nil
		}
		last := refs[len(refs)-1].ID
		cut := len(refs)
		for cut > 0 && refs[cut-1].ID == last {
			cut--
		}
		if cut > 0 {
			return refs[:cut], true, nil
		}
		// One id filled the whole batch; read on until it ends.
		cursor = refs[len(refs)-1]
	}
}

// groupRefs returns the distinct ids of refs in order, and each id's faces.
func groupRefs(refs []entity.Ref) (ids []string, faces map[string][]entity.Face) {
	faces = make(map[string][]entity.Face)
	for _, ref := range refs {
		if _, seen := faces[ref.ID]; !seen {
			ids = append(ids, ref.ID)
		}
		faces[ref.ID] = append(faces[ref.ID], ref.Face)
	}
	return ids, faces
}
