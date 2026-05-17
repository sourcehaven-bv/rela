// Package audit records every entity and relation write performed by
// the project as an append-only JSONL stream. It is *forensic*, not
// authoritative — the store is the source of truth. Audit records
// answer "what changed, when, and (best-effort) on whose behalf".
//
// The package exposes a single-method [Audit] interface plus three
// backends ([Nop], [Memory], [Filesystem]). Manager calls
// [Audit.Record] on every successful write; the per-call attribution
// ([principal.Principal] for "who", [WithTriggeredBy] for "what
// engine path") is carried via [context.Context] and read here.
//
// See [PLAN-XKMJ] in the tickets tree for the full design and the
// acceptance criteria each constructor / helper here satisfies.
package audit

import (
	"time"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// Op constants — the values that appear in Record.Op. Stable wire
// contract; downstream readers (jq, tail) match on these literals.
const (
	OpCreateEntity   = "create-entity"
	OpUpdateEntity   = "update-entity"
	OpDeleteEntity   = "delete-entity"
	OpRenameEntity   = "rename-entity"
	OpCreateRelation = "create-relation"
	OpUpdateRelation = "update-relation"
	OpDeleteRelation = "delete-relation"
)

// Subject identifies what an op acted on. Exactly one of {Type, ID}
// or {RelationType, FromID, ToID} is populated per record; readers
// switch on Kind.
//
//   - entity:   Kind="entity",   Type and ID populated.
//   - relation: Kind="relation", RelationType, FromID, ToID populated.
//
// Rename ops leave Subject zero and populate [Record.Before] /
// [Record.After] instead — the schema needs to carry both identities
// because the entity's ID is the thing changing.
type Subject struct {
	Kind         string `json:"kind"`
	Type         string `json:"type,omitempty"`
	ID           string `json:"id,omitempty"`
	RelationType string `json:"relation_type,omitempty"`
	FromID       string `json:"from_id,omitempty"`
	ToID         string `json:"to_id,omitempty"`
}

// Record is one audit row in the JSONL stream.
//
// Subject / Before / After are pointers so encoding/json can honor
// omitempty — non-pointer struct fields would marshal as
// `"subject":{}` even when zero. Rename ops populate Before/After
// and leave Subject nil; every other op populates Subject and
// leaves Before/After nil.
type Record struct {
	Time        time.Time           `json:"time"`
	Op          string              `json:"op"`
	Subject     *Subject            `json:"subject,omitempty"`
	Before      *Subject            `json:"before,omitempty"`
	After       *Subject            `json:"after,omitempty"`
	Principal   principal.Principal `json:"principal"`
	TriggeredBy string              `json:"triggered_by,omitempty"`
	Summary     string              `json:"summary,omitempty"`
}

// Audit is the consumer-side write surface every audit backend
// implements. Single method by design (CLAUDE.md "interfaces at the
// call site"); the no-return-value signature reflects the project
// rule that audit failure must never block an entity write (AC10).
// Backends self-log via slog.Error when a record cannot be persisted.
type Audit interface {
	Record(rec Record)
}
