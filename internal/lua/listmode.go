package lua

import "github.com/Sourcehaven-BV/rela/internal/entity"

// ListRows supplies the rows a list-document render displays.
//
// Defined here at the consumer (per CLAUDE.md "interfaces at the call site")
// so the lua package never learns about data-entry: the wiring site passes an
// adapter over a slice it has already resolved. Two methods because the
// bindings need both a cheap length (rela.document.count, without draining)
// and indexed access (row(i) / the rows() iterator).
//
// The rows are ALREADY resolved, ACL-scoped, filtered, sorted, and capped by
// the caller. The lua package performs no query of its own for them — that
// is the whole point of the seam. A list render must show what the on-screen
// view showed; re-deriving the set here would let the export drift from the
// view and escape the caller's row cap.
//
// At is called once per materialization, and a script may walk the set more
// than once, so implementations must be repeatable for the same index and
// must return nil when i is out of range.
type ListRows interface {
	Len() int
	At(i int) *entity.Entity
}

// ListSortSpec is one resolved sort criterion, in priority order.
type ListSortSpec struct {
	Property  string
	Direction string
}

// ListQuery is the read-only request context a list render receives: which
// list, over which entity type, under which filters and sort, and how the
// row cap applied. Plain data — a script may read it to title and annotate
// the export, but it is exposed through a frozen Lua table so it cannot be
// used as a back-channel to mutate the handler's state.
//
// Total is the row count BEFORE the cap; Rendered is what the script can
// actually see (== ListRows.Len()). They differ exactly when Truncated.
type ListQuery struct {
	ListID     string
	EntityType string
	Q          string
	Filters    map[string]string
	Sort       []ListSortSpec
	Total      int
	Rendered   int
	Truncated  bool
}

// ListRenderContext bundles everything a list-document render needs beyond
// the script path itself. Bundled rather than passed as three more
// parameters so [WithListDocumentMode] stays a two-argument option and
// script.Engine.ExecuteListDocument keeps a readable signature.
type ListRenderContext struct {
	ListID string
	Rows   ListRows
	Query  ListQuery
}
