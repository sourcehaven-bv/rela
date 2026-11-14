// Package records holds record types (TKT-J7Q9C4K): append-only, typed rows
// attached to an owner entity and stored beside the graph rather than in it.
// They serve the smaller time-series needs (readings, measurements,
// bookings) and ledger-style needs (balanced postings) of an application
// built on rela. They are not a time-series database or an accounting
// package.
//
// # Why this is not in the graph
//
// A row is cheap and numerous. An entity write pays for a file or row of
// its own, an audit line, a search update, versioning, automations and
// validation, none of which a reading needs. So rows stay out of the store,
// the audit log, the version sweep and the search index, as comments and
// channels do. A row is attributed on itself ([Record.RecordedBy]).
//
// # What this package holds
//
// The value types, the [Store] contract every backend implements, the
// generic aggregate a backend without [Aggregator] falls back on, and the
// [Service] that checks access, field values and the ledger rules on top of
// a Store. Transports call the Service, never a Store.
//
// The Store is the pluggable part. filerecords and sqlrecords (over pg and
// sqlite) are the shipped backends; a specialized one (a TSDB, a separate
// service) implements the same contract. Nothing in the contract needs the
// entity store: an aggregate receives an owner list the Service has already
// gated; it never queries the graph.
//
// # The contract every backend keeps
//
// recordstest.RunAll pins it once for all backends:
//
//   - Append is atomic: every row of the request is stored, or none is.
//   - A group key in [AppendRequest.Groups] is claimed by exactly one
//     append; a second append claiming it fails with [ErrGroupExists] and
//     stores nothing.
//   - List orders rows by (TS, ID) and pages with an opaque cursor that
//     stays valid while rows are appended. A row appended during paging
//     with a TS before the cursor is not returned by later pages.
//   - Rows are never changed. Only [Store.Expire], [Store.DropOwner] and
//     [Store.RekeyOwner] touch stored rows.
package records

import (
	"context"
	"errors"
	"time"
)

// MaxBatch bounds the rows of one append.
const MaxBatch = 1000

// MaxPage bounds the rows of one listed page.
const MaxPage = 1000

// DefaultPage is the page size when a query names none.
const DefaultPage = 100

// ReversalSuffix marks the group of a reversal: a reversal of group g is
// stored under group g + ReversalSuffix, so a group is reversed at most
// once. A caller-supplied group may not end in it.
const ReversalSuffix = "/reversal"

// Sentinel errors. Transports map them to their status codes.
var (
	// ErrNotFound is an unknown record type, or an owner the caller may not
	// read. The two are not told apart for an owner: whether an entity
	// exists is a secret.
	ErrNotFound = errors.New("records: not found")
	// ErrForbidden is an owner the caller may read but not update.
	ErrForbidden = errors.New("records: forbidden")
	// ErrInvalid is a request whose values do not fit the record type.
	ErrInvalid = errors.New("records: invalid")
	// ErrUnbalanced is a ledger transaction that does not sum to zero. It
	// wraps ErrInvalid.
	ErrUnbalanced = errors.New("records: transaction does not balance")
	// ErrClosed is a ledger row dated in a closed period. It wraps
	// ErrInvalid.
	ErrClosed = errors.New("records: period is closed")
	// ErrGroupExists is an append that claims a group already taken, such
	// as a transaction id used before or a group reversed before.
	ErrGroupExists = errors.New("records: group already exists")
)

// Record is one stored row.
type Record struct {
	// ID is unique and assigned by the Service. IDs sort by creation.
	ID string `json:"id" yaml:"id"`
	// Type is the record type key.
	Type string `json:"type" yaml:"type"`
	// Owner is the id of the owner entity, the whole face family.
	Owner string `json:"owner" yaml:"owner"`
	// TS is when the row applies: the reading time, the booking date.
	TS time.Time `json:"ts" yaml:"ts"`
	// RecordedAt is when the row was appended.
	RecordedAt time.Time `json:"recorded_at" yaml:"recorded_at"`
	// RecordedBy is the user that appended the row.
	RecordedBy string `json:"recorded_by,omitempty" yaml:"recorded_by,omitempty"`
	// Group is the ledger transaction the row belongs to; empty for a
	// type without a ledger.
	Group string `json:"group,omitempty" yaml:"group,omitempty"`
	// Reverses is the group this row reverses; empty otherwise.
	Reverses string `json:"reverses,omitempty" yaml:"reverses,omitempty"`
	// Fields are the declared fields with a value. Values are string,
	// int64 or bool; a decimal is its canonical text.
	Fields map[string]any `json:"fields" yaml:"fields"`
}

// GroupKey names one group of one record type.
type GroupKey struct {
	Type  string
	Group string
}

// AppendRequest is one atomic append.
type AppendRequest struct {
	Records []Record
	// Groups must not exist yet; the append claims them.
	Groups []GroupKey
}

// ListQuery selects rows of one type, ordered by (TS, ID).
type ListQuery struct {
	Type string
	// Owner selects one owner's rows. Empty selects every owner, which
	// only the Service's ledger reads use together with Group.
	Owner string
	// Group selects the rows of one group.
	Group string
	// From and To bound TS to [From, To). Zero is unbounded.
	From, To time.Time
	// After is the cursor of the previous page.
	After string
	// Limit is the page size, at most [MaxPage]. Zero means [DefaultPage].
	Limit int
}

// Page is one page of rows. Next is empty on the last page.
type Page struct {
	Records []Record `json:"records"`
	Next    string   `json:"next,omitempty"`
}

// Store is the storage contract. See the package doc for what every
// implementation keeps.
type Store interface {
	// Append stores the rows of r atomically and claims its groups.
	Append(ctx context.Context, r AppendRequest) error
	// List returns one page of rows.
	List(ctx context.Context, q ListQuery) (Page, error)
	// Owners returns the ids of the owners with rows of type typ, sorted.
	Owners(ctx context.Context, typ string) ([]string, error)
	// Types returns the record types that have stored rows, sorted. It
	// includes a type the schema no longer declares: such rows stay with
	// their owner, and an export or copy must still reach them.
	Types(ctx context.Context) ([]string, error)
	// RekeyOwner moves the rows of oldID to newID, for an entity rename.
	RekeyOwner(ctx context.Context, oldID, newID string) error
	// DropOwner deletes the rows of id, for an entity delete.
	DropOwner(ctx context.Context, id string) error
	// Expire deletes the rows of type typ with TS before before, and
	// returns how many it deleted.
	Expire(ctx context.Context, typ string, before time.Time) (int64, error)
}

// Aggregator is an optional [Store] capability: a backend that folds rows
// natively. The Service uses it when the backend has it and
// [AggregateGeneric] otherwise. Owners in the query are already gated.
type Aggregator interface {
	Aggregate(ctx context.Context, q AggregateQuery, scale int) ([]Point, error)
}
