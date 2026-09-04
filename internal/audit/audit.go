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

	// OpDeniedWrite records a write attempt that the ACL refused.
	// Subject names the would-be target (entity type or relation type);
	// Summary carries the deny rule_kind / rule_id / reason and the
	// attempted op (one of the Op* above). Forensic: denials answer
	// "what did this user try to do that they weren't allowed to?"
	OpDeniedWrite = "denied-write"

	// OpACLBypass records a write that skipped the ACL deny because it ran
	// through an elevated automation handle (rela.bypass_acl — TKT-D8T148).
	// Subject names the target; Summary carries acl_bypass=true + the genuine
	// write op. The Principal is the REAL triggering identity (not a system
	// user) so "who caused this elevated write" stays answerable; TriggeredBy
	// carries automation:<name>. Forensic: isolate every elevated write with
	// `op == "acl-bypass"`.
	OpACLBypass = "acl-bypass"

	// OpACLBypassRead records that an elevated automation closure
	// (rela.bypass_acl) performed at least one RAW READ — TKT-ACSBSA.
	//
	// Separate from OpACLBypass because the two answer different questions
	// and have different blast radii: a bypass WRITE changed the graph and
	// names its subject; a bypass READ changed nothing but may have
	// disclosed anything. Folding reads into "acl-bypass" would silently
	// change what every existing `op == "acl-bypass"` query means.
	//
	// Emitted ONCE PER CLOSURE, not per read: one admin.list_entities can
	// traverse the entire graph, so a per-row record would be an unbounded
	// synchronous write on a read path. Subject is deliberately EMPTY and no
	// entity data is recorded — the read set is unbounded, and logging it
	// would copy ACL-protected content into the audit log, a wider
	// disclosure than the read itself. Summary carries acl_bypass_read=true
	// plus the admin bindings used. The Principal is the REAL triggering
	// identity; TriggeredBy carries automation:<name>. Isolate elevated
	// reads with `op == "acl-bypass-read"`.
	//
	//nolint:gosec // G101 false positive: an audit op name, not a credential.
	OpACLBypassRead = "acl-bypass-read"

	// OpACLQuery records an execution of an effective-access query
	// (`rela acl who-can`) — a read that produces a confidentiality
	// attestation: who may act on an entity, and by which roles, groups and
	// graph edges (TKT-M86UY8, CONTROL-8-15).
	//
	// Subject names the entity queried; Summary carries the verb. The RESULT
	// is deliberately NOT recorded: the answer is a list of principals and
	// the routes by which they hold access, and copying that into the audit
	// log would duplicate the disclosure rather than record it — the same
	// rule OpACLBypassRead applies to elevated reads.
	//
	// Worth logging because the output is exactly the reconnaissance an
	// attacker with shell access wants before choosing a target, and exactly
	// the question an investigator asks afterwards. Isolate with
	// `op == "acl-query"`.
	OpACLQuery = "acl-query"

	// OpPurgeVersion records an operator hard-delete of version snapshot rows
	// (TKT-BW6UUL) — the deliberate, irreversible exception to append-only
	// history, for compliance redaction. Subject names the entity/relation whose
	// lineage was purged; Summary carries the count, the vseq(s) or content-hash
	// targeted, and the operator's --reason. The purged CONTENT is never recorded
	// (that would defeat the purge); this record is the surviving forensic trail
	// showing who purged what and why. Isolate with `op == "purge-version"`.
	OpPurgeVersion = "purge-version"

	// OpCopyState records one invocation of a declared COPY DEFINITION —
	// a mapped write of one entity content state (face) into another
	// (TKT-C1XUA8). Subject names the TARGET face; Summary carries the
	// definition name, the source and target faces, and whether the target
	// was created.
	//
	// Shaped after OpPurgeVersion: it records WHAT was done to WHICH
	// subject and never the copied content. Unlike purge, it does NOT
	// bypass the Manager — a copy IS an entity write, so it goes through
	// the same audit hook that reads attribution from ctx and cannot be
	// forged by a caller.
	OpCopyState = "copy-state"

	// OpDataMigration records one applied data-migration file (TKT-0C57FS):
	// a bulk store-level rewrite that deliberately bypasses the
	// entitymanager (so no per-entity audit records exist for it). Summary
	// carries the file name, the from→to shape hashes and per-step counts;
	// migrated CONTENT is never recorded. Isolate with
	// `op == "data-migration"`.
	OpDataMigration = "data-migration"

	// OpDataGC records one garbage-collection pass (TKT-0C57FS) that
	// deleted schema-orphaned data after the drift grace period — deleted
	// property values, entities or relations of types the schema no longer
	// declares. Summary carries the ledger keys and counts, never content.
	// Isolate with `op == "data-gc"`.
	OpDataGC = "data-gc"
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
// omitempty — non-face struct fields would marshal as
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

// Audit is the package's published sink shape — not a consumer-side
// abstraction. Three sibling backends ([Nop], [Memory], [Filesystem])
// implement it; consumers (today: entitymanager.Deps.Audit) take it
// by value. This is the io.Writer pattern, not the Repository
// pattern: the interface IS the contract, and moving it
// consumer-side would force every consumer to redeclare the same
// single method.
//
// The no-return-value signature reflects the project rule that
// audit failure must never block an entity write — backends
// self-log via slog.Error when a record cannot be persisted.
type Audit interface {
	Record(rec Record)
}
