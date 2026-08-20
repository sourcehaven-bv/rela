---
id: RES-273DPJ
type: research
title: How should rela's user-data migration system be authored, executed, and bookkept?
summary: 'Agent survey (input for Jeroen''s parallel track — owner: Jeroen). Recommends declarative ops files + hybrid execution (automation-suppressed Manager writes for value ops, store-contract cascade ops for re-keys) + state.KV applied-ledger, dry-run default, system:migration principal; Lua and auto-diff rejected as the authoring model.'
status: done
---

> **Owner: Jeroen** (architect discussion 2026-08-19). This is the agent's
> completed survey, restored as INPUT for Jeroen's parallel design track —
> not a settled direction. Treat the recommendation as a second opinion;
> the precedent inventory in Context is useful regardless of direction.

Research for FEAT-T3EF5A (DEC-0VGTF3). Survey run 2026-08-19 against
post-TKT-80EWGM develop.

## Problem

A user metamodel edit that re-means stored data — rename a property, rename an
entity type, change an enum value, later re-key a pointer or change axes —
silently strands everything written under the old schema. There is no system to
carry the user's DATA along with their schema evolution; `internal/migration`
migrates only the schema-YAML *format* when rela itself evolves. DEC-0VGTF3
makes this system the designated remedy for content-states schema evolution
(pointer re-key becomes one migration op; FEAT-9CD2MX v1 ships only
undeclared-pointer *detection* pointing at it), so the shape needs deciding:
authoring model, execution path, bookkeeping.

## Context

**What `internal/migration` is, and why it cannot carry data migrations.** A
registry of `Migration{Name, Description, FileTypes, Detect, Apply}`
implementations over `yaml.Node` ASTs (comment/format-preserving), applied by a
runner to the three config files (metamodel/schema.yaml, data-entry.yaml,
acl.yaml); optional `MetamodelAware` injection (`migration.go`, `runner.go`). It
has NO store access, no per-row iteration, no backend awareness, and no
applied-state ledger — none needed, because `Detect` is idempotent on the
artifact itself (deprecated syntax is self-evident in the document). Data
migrations invert every one of those properties: once the metamodel is edited,
the *data* no longer says what it should become (a renamed property is
indistinguishable from a deleted one by inspection), so detection-from-artifact
collapses and an explicit authored intent + applied ledger is mandatory.

**Existing data-touching precedents:** `rela renumber` (plan-then-execute bulk
write THROUGH Manager, --dry-run, per-row audit); `internal/renametype` (type
rename on raw FS — silently doesn't touch pg rows: the cautionary tale for
ad-hoc per-op tooling); `Manager.ApplyEntity` (full ACL+validation+audit, NO
automation/cascade — the bulk-write primitive shape, TKT-78R2YB rationale
carries over); `Manager.RenameEntity` store cascade (the model for key-shaped
ops); `history-purge` (operator-shell trust, dry-run default, advisory-lock
exclusion); `internal/state.KV` (cross-backend durable KV outside the change
feed — the applied-ledger home); `system:*` principal precedent for
`system:migration`.

## Options

### Authoring model

**A1 — Declarative ops files (recommended).** Named migration files of typed
ops: `rename_property`, `map_enum_values`, `set_default`, `rename_type`,
`rekey_id_prefix`, `rekey_pointer`. Pros: analyzable, dry-runnable, per-backend
pushdown possible, matches the project's declarative-only precedent line (world
templates, copy definitions), reviewable. Cons: vocabulary must be grown
deliberately; genuinely odd transformations need a new op rather than an escape
hatch. Effort: m.

**A2 — Lua migration scripts** through `WriteDeps` (scheduler-style). Pros:
exists informally today; arbitrary power. Cons: not analyzable — dry-run
degrades to "run it against a copy"; idempotency unverifiable; no per-backend
pushdown (N Lua round-trips vs one SQL UPDATE); directly against the
declarative-only precedent line. Effort: s to bless, high permanent cost.
Rejected as the system; unblessed scripts remain the power-user escape they
already are.

**A3 — Auto-derived metamodel diff.** Pros: zero authoring. Cons: the classic
ambiguity — rename vs delete+add is NOT inferable from a diff, and a wrong guess
destroys data; requires retaining prior metamodel versions. Viable only as an
ASSISTANT that *proposes* an A1 ops file for the operator to review (`rela
data-migrate init --from-git HEAD~1`-ish). Effort: l for the assistant; unsafe
as the system.

### Execution path

**B1 — Everything through entitymanager** (an ApplyEntity-shaped bulk primitive:
validation + audit + events, automations suppressed, `system:migration`
attribution). Pros: audit/versioning/search-index correctness for free. Cons:
key-shaped ops (type rename, id-prefix re-key, pointer re-key) are not
expressible as per-row upserts — they need cascade semantics only the store can
provide; per-row cost on pg.

**B2 — Direct store writes / per-backend SQL pushdown.** Pros: fast, atomic on
pg. Cons: bypasses audit, versioning, and store events — the search index
silently goes stale, the change feed drops the writes; and it reproduces the
renametype drift problem per backend. Rejected as the general path.

**B3 — Hybrid (recommended).** Value-shaped ops (property rename per row, enum
mapping, defaults) run through the Manager bulk primitive — audited (per-row
records + one summary record naming the migration), evented, versioned,
automation-suppressed, ACL-exempt at the operator boundary exactly like
`history-purge`/`db migrate`. Key-shaped ops (type rename, id re-key, pointer
re-key) become **store-contract operations with cascade semantics** — the
`RenameEntity` model — implemented per backend, pinned by `storetest`
conformance cases before the second backend implements (the TKT-DOFYR1
discipline), and emitting the same events/version captures rename does today.
`renametype`'s FS-side work (metamodel text, directories, templates) folds into
the runner's project-file phase, closing its pg gap.

### Bookkeeping / operations

- **Applied ledger in `internal/state.KV`** (one mechanism, both backends; pg
gets it multi-process-safe via `state_kv`): migration name + content hash +
applied-at + outcome. Refuse to re-apply a changed-hash migration.
- **Dry-run by default; explicit `--apply`** (history-purge precedent). Dry-run
reports per-op row counts + example ids — the same shape as the
undeclared-pointer analyze finding, deliberately.
- **Failure story per backend, stated honestly:** pg wraps a migration in the
native transaction (rollback on failure) under the sweep-family advisory lock
(mutual exclusion with sweep/purge); fs is best-effort under the Tx write mutex
with ordered writes + idempotent re-run as the recovery path — same accepted
asymmetry as fsstore copy atomicity, documented not papered over.
- **Trust boundary:** operator shell, CLI-only (`rela data-migrate
detect|plan|apply`), no ACL evaluation, full audit trail.

## Recommendation

**A1 + B3 + the bookkeeping above.** Declarative ops files; hybrid execution
(Manager-shaped bulk writes with automations suppressed for value ops;
storetest-pinned store cascade ops for re-keys); `state.KV` applied ledger;
dry-run default; `system:migration` principal via the entitymanager attribution
boundary; operator-shell trust like `db migrate`. A3 returns later as an
authoring assistant only.

Tradeoffs accepted: the op vocabulary is a maintained surface (each new op is a
deliberate design act — which is the point); per-row write cost for value ops on
pg (bounded by graph sizes rela targets; correctness of audit/versions/search-
index wins); no arbitrary-code escape hatch (an odd case becomes a new declared
op, not a script).

Sequencing note: the store-contract re-key ops are where FEAT-9CD2MX's pointer
re-key lands; nothing in TKT-DOFYR1 gates on this system (detection-only per
DEC-0VGTF3), but DOFYR1's storetest discipline and its state-enumeration surface
are direct inputs to the re-key op design.
