---
id: RES-4ILUJZ
type: research
title: 'pgstore content versioning: history-table storage shape + schema-drift handling (content-addressed schema snapshots)'
summary: v1 = full-snapshot entity_versions rows written best-effort at the entitymanager boundary (B1 — non-atomicity accepted as a rare single-version gap, self-healing under full snapshots); schema drift handled by a content-addressed render-schema *projection* (property/display/enum/custom-type config only, not automations/validations — better dedup), versions-only; entities-only for v1.
status: done
---

## Problem

TKT-9INY0Y adds time-machine content versioning to the pgstore backend. Scoping
settled the *what* (list/view/diff past entity versions with Principal
attribution) and several structural decisions. This research settles the *how*
for storage shape, write-path/atomicity, and schema-drift handling.

> **NOTE:** The **Decisions (post-review)** section near the bottom is the final, authoritative outcome and supersedes the "Recommendation" text where they differ. The Options analysis is retained as the reasoning trail. Read the **Scenarios** section first — it is the motivation the design must serve, and every design choice below is justified against it.

## Scenarios — who cares, what they care about, and why

The feature exists to serve these actors. Each design decision below is
justified by (or checked against) one of them.

### S1 — Compliance / auditor (primary driver)
**Wants:** "Show me exactly what `REQ-42` said on 2026-03-01, and everyone who
changed it since, with names." — the *faithful* historical state of an entity as
of a date, plus the full attribution chain. **Why:** regulatory / forensic
accountability (this project itself runs an ISO-27001 ISMS in-tree;
audit-time-travel is a named future need in `IDEA-CQMKMD`). An approximate or
mis-rendered old state is worse than none — it's evidence. **What it demands of
the design:**
- **Schema-faithful rendering is non-negotiable, not nice-to-have** → justifies C1 (content-addressed schema projection). An old version rendered under today's (drifted) schema would misstate the record.
- **Attribution is non-negotiable** → the `Principal` (user + tool) and `triggered_by` on every version. This is *the* forensic payload.
- **Deleted entities must remain answerable** → an auditor asks about an entity that has since been deleted. Version history must survive entity deletion (the version rows are not cascade-deleted with the entity). Interacts with the existing tombstone machinery. **Open scope question flagged below.**

### S2 — Editor recovering from a bad change (primary driver)
**Wants:** "An automation / an LLM agent / a teammate mangled this entity — what
did it look like before, and let me put it back." — view + diff the prior
version, **and restore it**. **Why:** confidence and undo in a shared
multi-writer pg deployment, where fsstore users had `git checkout` / `git
revert` for free. Without this, a bad automated write in pgstore is
unrecoverable — a real regression vs. the fsstore experience the ticket is
trying to reach parity with. **What it demands of the design:**
- **Restore / revert-to-version is a first-class need for this actor**, and the current v1 scope (list / view / diff) **silently omits it**. See "Scope gap" below — this is the most important scenario finding.
- View + diff must be good enough to *decide* what to restore (readable prose diff for content, field-level diff for properties).

### S3 — Reviewer / collaborator (primary driver)
**Wants:** "What changed in this entity between last week and now, and who did
each change?" — a readable diff and a who/when timeline over a range. **Why:**
review changes in a pg deployment where there's no git log to `git blame` / `git
diff`. **What it demands of the design:**
- **Diff quality** — prose/line diff for markdown `content`; structured field-level diff for JSONB properties (not a text diff of serialized JSON).
- **Timeline with attribution** — the `entity_versions` list ordered by `seq`, each row showing Principal + triggered-by + timestamp.
- **Related-entity titles must render in a diff/timeline** → justifies the **whole-render-projection** granularity (snapshot all types' render config, not just the version's own type) so a diff can show a linked entity's title as it was.

### Secondary (rides along, not a design driver)
- **Debugging an automation/agent** ("which automation keeps rewriting this?") is served for free by S3's timeline + `triggered_by` attribution — no extra design.

### Scope gap surfaced by the scenarios: RESTORE
S2 wants **restore-to-version**, which list/view/diff does not provide. Options:
- **(a)** Ship read-only history in v1 (list/view/diff), defer restore to a fast follow-up. Lower risk; but S2 — parity with fsstore's `git revert` — is a primary driver, so this leaves the headline "recover from a bad change" story half-built.
- **(b)** Include restore in v1: "restore" is just a normal `UpdateEntity` through `entitymanager` whose payload is a past version's `{content, properties}` — so it goes through ACL + validation + audit + *creates a new version* (the restore itself is attributed and versioned). Mechanically small on top of the read path; the nuances are (i) restoring a *deleted* entity = a create, and (ii) relations aren't versioned in v1 so a restore only restores entity content/props, not its relation set as-of-then. **Recommend (b) for entity content/props**, explicitly scoping out relation-set restore (consistent with D1 entities-only).

## Context

### Write path & attribution (codebase survey)

- `entitymanager.Manager` is the single write choke-point. Every op does **ACL → validate → store write → `Audit.Record(...)`**, where the audit call happens *after* the store call returns (`internal/entitymanager/manager.go`, `audithook.go`). Audit is fire-and-forget: `Audit.Record` returns nothing, by explicit design ("audit failure must never block an entity write", `internal/audit/audit.go:98`).
- **The boundary already has everything a version row needs, in memory:** Principal (`principal.From(ctx)`), triggered-by (`audit.TriggeredByFrom(ctx)`), and — on update/delete — the full **before-state** `*entity.Entity` (loaded at `manager.go:489` for update, `:563` for delete; rename re-fetches post-state). Create has no before; the "after" is the written entity.
- **But it is strictly post-commit / non-transactional.** pgstore opens its *own* `tx` per write and commits before returning (`pgstore/entity.go:213,254,292,363`); `pgx.Tx`/`DBTX` are pgstore-private and never exposed. There is **no** existing example of entitymanager doing anything transactional across the `store.Store` interface. A version write at the boundary runs only *after* the entity is durably committed — a crash in between persists the entity change with no version row (exactly audit's tolerance today).
- The `store.Store` write methods (`store.go:251-265`) take **no metadata**. The only intra-tx atomic option is a hook *inside* pgstore. pgstore already writes extra rows inside the same write tx (tombstone precedent, `pgstore/tombstone.go`, `0003_sync.sql`).

### Schema / hashing (codebase survey)

- `Metamodel` is one YAML-tagged struct (`internal/metamodel/types.go:15`), loaded from disk (`loader.go:42`). **No whole-metamodel serializer or persisted schema hash exists.**
- Two reusable precedents: **`internal/canonical`** (TKT-8FSBGB) — length-prefixed SHA-256 streaming with cross-backend type normalization; machinery generalizes but there is no `HashMetamodel`. And **`internal/openapi` `computeMetamodelHash`** (`generator.go:155-224`) — a deterministic traversal that already hashes *exactly the render-relevant projection* (per entity type: label/plural + per-property name/type/required/list/enum `values`; relations from/to; custom-type values). It's lossy for a general schema hash but is **almost precisely the render-schema projection** we want — a proven model to build on, just made lossless (add `display_property`, property order, widget/format) + length-prefixed.
- pgstore's `schema_version` table is DDL migration versioning (an int), unrelated to metamodel content (`pgstore/migrate.go:62`). Metamodel is always read from disk.
- **Concrete drift vectors** (`entity_def.go:158,244`): `display_property`, enum `values`, property `type`, and `GetPrimaryProperty`'s required-string logic all feed `DisplayTitle`. Rendering an old version under a *new* schema can mis-title it, mis-stringify enum/int values, or resolve the wrong primary property.
- **Snapshot shape:** domain form `entity.Entity{ID, Type, Properties map[string]any, Content string}`. At-rest bytes differ across backends, so a snapshot captures the **normalized domain form** (like `canonical`), not raw stored bytes.

### External prior art

- **PostgreSQL audit patterns** commonly store `old_values`/`new_values` as JSONB. Full-snapshot vs. delta: deltas compact + scannable per-field; full snapshots simplify point-in-time reconstruction. ([CYBERTEC](https://www.cybertec-postgresql.com/en/row-change-auditing-options-for-postgresql/))
- **SQL Server system-versioned temporal tables** *force the history table to stay schema-aligned* with the live table — old rows are back-filled to the new column shape. ([MS Learn](https://learn.microsoft.com/en-us/sql/relational-databases/tables/changing-the-schema-of-a-system-versioned-temporal-table)) Precisely the drift trap to avoid; strong evidence *for* content-addressed schema snapshots over DB-native temporal tables.
- **Document-store schema versioning** (MongoDB) tags each doc with its schema version so readers render faithfully. ([ELCA](https://medium.com/elca-it/schema-versioning-and-upgrade-in-document-store-implementation-with-java-mongodb-and-springdata-bbd4062c9819)) The content-addressed hash is the same idea with dedup for free.

## Options

### A. Storage shape

- **A1 — Full snapshot per version (chosen).** One `entity_versions` row per write with the complete normalized `{content, properties, type}`; diffs computed on read. Pros: trivial reconstruction (**directly enables S2 restore** — a version *is* a restorable state), simple diff (S3), sidesteps schema-reconstruction hazards, dedup-friendly by content hash. Cons: storage grows (retention cap mitigates). Effort: low.
- **A2 — Delta / patch chains.** Compact, but fragile chained reconstruction, worse with mid-chain schema change, more code. Effort: medium–high. Rejected.
- **A3 — Postgres-native temporal.** Forces history schema-alignment (the trap → breaks S1 faithful rendering), heavier live table, no home for attribution (breaks S1/S3). Rejected.

### B. Where the version row is written (atomicity)

- **B1 — Entitymanager boundary, post-commit, best-effort (chosen for v1).** Version row written from a hook next to `Audit.Record` using in-memory before/after + Principal. Pros: all data in scope (Principal for S1/S3), consistent with audit, store stays a black box, no pgstore API change, backend-agnostic. Cons: non-atomic — a crash between commit and version-write drops that one version. Assessed low-severity vs. the scenarios: S1 auditor could in principle miss one intermediate revision, but the loss is a single self-healing gap, not corruption or mis-attribution of surviving rows. Effort: low.
- **B2 — Inside pgstore's tx, Principal via ctx.** Fully atomic but threads principal into the store, reversing the "store never learns Principal" decision. Effort: medium.
- **B3 — Hybrid: content intra-tx, attribution at boundary.** Content atomic; attribution best-effort join. The right *target* if S1's atomicity ever proves to matter. Effort: medium.

### C. Schema-drift handling

- **C1 — Content-addressed schema snapshot, versions reference it (chosen), with a render-schema *projection* (not the whole metamodel).** New `schema_versions(hash PK, snapshot, captured_at)`. On write, hash the **render-relevant projection** of the metamodel (property defs incl. type/required/list/enum `values`, `display_property`, property order, custom-type values, widget/format) — deliberately **excluding** the churny non-render parts (automations, validations, cascade rules, colors, id-config). Insert if absent (dedup by hash); `entity_versions.schema_hash` points at it; view/diff render against the stored projection. **Directly serves S1** (faithful historical rendering) **and S3** (correct diff field types). Pros: faithful rendering; **projection makes the hash stable against automation/validation edits → far better dedup**; avoids the temporal-table trap. Cons: must define the projection precisely (bounded; `openapi` already models ~90%). Effort: medium.
- **C2 — Extend schema_hash to live entities.** Unlocks "which entities predate migration X" + faithful rendering everywhere, but touches the hot `entities` table and every live write path. Deferred to follow-up; design `schema_versions` so it *can* be added without a migration rewrite.
- **C3 — Ignore drift, render against live schema.** Breaks S1 (auditor gets a mis-stated record) and S3 (wrong diff). Rejected.

### D. Relations (secondary)

- **D1 — Entities only for v1 (chosen).** Relation history deferred. Note the cost against the scenarios: S1/S3 see entity content+property history but *not* how an entity's relation set changed over time, and S2 restore recovers content/props but not the as-of relation set. Accepted for v1; called out explicitly so the scenarios' partial coverage is honest.
- **D2 — Embed relations in the entity snapshot.** Rejected: relations are shared between two entities (independent rows with their own PK), so embedding silently mutates the un-written entity's history or duplicates. First-class beats embedding if relation history is ever pursued.

## Decisions (post-review) — AUTHORITATIVE

Settled with the user after reviewing the survey and the scenarios.

**0. Scenarios (S1 auditor, S2 recover-bad-change, S3 reviewer) are the
motivation** and are recorded above. The design is justified against them; the
one scope change they force is **restore** (below).

**1. Atomicity → B1 (boundary-only, best-effort). Non-atomicity accepted.** Loss
window is microseconds between `tx.Commit()` and the version-write round-trip; a
crash there loses **one** historical snapshot (not content, not corruption, not
mis-attribution). Full snapshots (A1) make the gap self-healing on the next
write. Matches rela's existing best-effort-audit tolerance and the git mental
model. Keep the hook shaped so it *could* later move into pgstore's tx (→ B3) if
S1 ever demands strict continuity.

**2. Schema drift → C1 with a render-schema PROJECTION, versions-only.** Store
only render/type-relevant bits, not the whole metamodel — a
**dedup-correctness** win (hash stays stable across automation/validation
edits). Projection (finalize in planning): per entity type — property defs
(name, type, required, list, enum `values`, format/widget), `display_property`,
property order, primary-property inputs; plus referenced custom-type value
lists. Model on `openapi.computeMetamodelHash`, made lossless + length-prefixed
via `internal/canonical`. **Granularity: whole render-projection (all types)**
so S3 diffs can render related-entity titles. Versions-only; design
`schema_versions` so live entities can reference it later (C2) without a
migration rewrite.

**3. Storage → A1 full snapshots. 4. Scope → D1 entities only.**

**5. RESTORE is IN v1 (forced by S2).** Restore-to-version = an
`entitymanager.UpdateEntity` (or create, if the entity was deleted) whose
payload is a chosen past version's `{content, properties}`; it flows through ACL
+ validation + audit and **produces a new version** (the restore is itself
attributed and versioned — no history rewriting). Scoped to entity
content/properties only; the as-of **relation set is not restored** (consistent
with D1). Read-only-first (a) was considered and rejected because S2
(fsstore-`git revert` parity) is a primary driver, not a nice-to-have.

**6. Deleted-entity history survives (forced by S1).** `entity_versions` rows
are **not** removed when the entity is deleted — an auditor must still answer
"what did the now-deleted `REQ-42` say?", and S2 restore-of-a-deleted-entity
depends on it. Delete itself records a final version (the pre-delete state).
Interacts with tombstones; retention policy still applies. Finalize the exact
delete/tombstone interaction in planning.

## Follow-ups to capture as separate tickets/ideas

- **C2** — live entities reference `schema_hash` ("which entities predate migration X" + faithful live rendering). Cool-to-have, not v1.
- **B3** — harden version-write atomicity by capturing content inside pgstore's tx, if B1's rare gap ever proves to matter for S1.
- **Relation history** (first-class, triple-keyed) — would complete S1/S3 relation-change visibility and S2 relation-set restore.
