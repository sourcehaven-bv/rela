// Package reconwire is the wire format of POST /api/v1/_reconcile
// (FEAT-CMVZQFI): the request a peer sends and the answer it gets. The
// server handler in dataentry and the client in reconclient share it, so a
// client needs only the reconcile protocol, not the data-entry API contract
// in apiwire. Problems are RFC 9457 bodies like every other API error.
package reconwire

// Request is the body of POST /api/v1/_reconcile (TKT-VP02WHG): the
// element ranges a peer asks about, answered in order.
type Request struct {
	Ranges []Range `json:"ranges"`
}

// Range is one range of a [Request].
//
// Kind is "entity" or "relation". Entities are the entity types in scope;
// Relations the relation types, set only for a relation range, which also
// needs Entities: a relation is in range only when both endpoints are rows
// of named types the caller may read.
//
// The lower bound is Lo (inclusive) or After (exclusive), never both; After
// takes the Last of a previous list answer. Op is "fingerprint" or "list";
// Limit is the page size of a list and must be zero for a fingerprint.
type Range struct {
	Kind      string   `json:"kind"`
	Entities  []string `json:"entities"`
	Relations []string `json:"relations,omitempty"`
	Lo        string   `json:"lo,omitempty"`
	After     *string  `json:"after,omitempty"`
	// Hi is the exclusive upper bound.
	//
	// Nil: accepted. The range then has no upper bound.
	Hi    *string `json:"hi,omitempty"`
	Op    string  `json:"op"`
	Limit int     `json:"limit,omitempty"`
}

// Response answers a [Request]: one result per range, in
// request order.
type Response struct {
	Ranges []Result `json:"ranges"`
}

// The statuses of a [Result].
const (
	// StatusOK is an answered range.
	StatusOK = "ok"
	// StatusTooLarge is a range that holds more rows than one call
	// covers. Split it by type, then by address.
	StatusTooLarge = "too_large"
	// StatusNotRun is a range the request's deadline cut off. Ask again.
	StatusNotRun = "not_run"
)

// Result is the answer for one range. Status is one of the
// Status* constants; the other fields are set only when it is "ok".
//
// A fingerprint answer sets Count and Fingerprint: the number of elements
// and the sum of their fingerprints modulo 2^128, as 32 hex digits.
//
// A list answer sets Elements, Last and More. Last is the address of the
// last row the page covered, which may be past the last element (a row
// that was gone when loaded is left out); More reports whether the page
// stopped at the limit. Resume with After set to Last while More is true,
// whatever the number of elements.
type Result struct {
	Status      string    `json:"status"`
	Count       *uint64   `json:"count,omitempty"`
	Fingerprint string    `json:"fingerprint,omitempty"`
	Elements    []Element `json:"elements,omitempty"`
	Last        string    `json:"last,omitempty"`
	More        bool      `json:"more,omitempty"`
}

// Element is one element of a list answer: its address and its
// view hash (64 hex digits), the canonical digest of the row as the caller
// sees it.
type Element struct {
	Address string `json:"address"`
	Hash    string `json:"hash"`
}
