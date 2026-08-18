---
id: PLAN-LYXAI6
type: planning-checklist
title: 'Planning: Sync becomes a client of the authorized API: read + write through /api/v1, retire the private sync record channel'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Scope reset (2026-08-15) — supersedes the "redact the fetch" plan

The original plan (and its completed design-review, 6 findings) was scoped to
redacting `handleSyncGet` in place while keeping both sync handlers. That
approach was superseded twice — first by "full write-unify," then rejected in
favor of the **fancy-browser** model (see TKT-8P1TM7 design trail). This
checklist is re-planned against that model; a **fresh design-review is
required** (the prior one does not cover the new write path or temp-id
reconciliation).

**What carries over from the old design-review:**
- **RR-ATFNM1** (body redaction OUT OF SCOPE) — still holds: no body guards
exist on any read path; the `/api/v1` GET this ticket now reuses ships the full
`Content` today. Body-redaction remains a separate cross-path ticket.
- **RR-DGBVFO / RR-4D4UBM** (client contract: re-bootstrap on access-scope
change; manifest→fetch 404 = skip+advance) — still hold; now apply to the
`/api/v1` fetch.

**What is obsoleted by the reframe:**
- **RR-596TYU** (fail-closed relation fetch in `handleSyncGet`) — moot;
`handleSyncGet` is retired. `/api/v1` relation reads already enforce their own
gate.
- **RR-IWXMDW / RR-L0BN94** (canonical-hash ETag invariant / oracle) — moot for
the read path; the fetch now uses the `/api/v1` ETag. Pull-apply still uses the
canonical hash for LOCAL conflict detection (a different, replica-internal
concern), so the "hash the raw record, not the redacted body" invariant survives
there — to be re-confirmed in the new design-review.

## The model to plan against (three flows)

1. **Read (pull-fetch) → `/api/v1` GET.** Inherits row-gate + `visible:`
redaction. Retires `handleSyncGet`.
2. **Pull-apply → replica's own local store, as a PATCH (browser-like).** No
automations (log replay, idempotency). Uses `_redacted` (DEC-T0XIWQ) to
disambiguate present→patch / named-in-`_redacted`→leave / in-neither→unset, so
redacted ≠ deleted with no leak and no feed tombstones. Baseline for conflict
detection is the **remote's ETag** (not a locally recomputed canonical hash).
3. **Push → remote SPA `/api/v1` (unchanged).** Automations fire;
`validateFieldWrite` enforced; **remote mints ids** — replica creates under a
temp id, adopts the remote id on ack, renames its local doc, remaps references.

Kept: the `/api/sync/manifest` change feed (content-free, row-gated, cursor +
tombstones). Retired: `handleSyncGet`, `handleSyncPut`, the sync write-ACL
bypass.

## Understanding

- [x] Root cause understood (parallel channel evaluates auth independently; read
redaction bypass + write field-ACL bypass are its two symptoms)
- [x] Existing sync client surface mapped (`internal/cli/sync`:
client.go path builders, push/pull/force, conflict decode)
- [x] `/api/v1` read + write surface mapped (done — investigation task; GET
redaction, PATCH partial-body, create mints id, validateFieldWrite)
- [x] `ApplyEntity` semantics confirmed for pull-apply (suppression, id-preserve,
canonical If-Match) and the splice-onto-raw change identified

## Approach

- [x] Read path: point the sync client fetch at `/api/v1` GET; delete
`handleSyncGet`
- [x] Pull-apply: build a `Patch` from fetched fields and merge onto the raw
local record (splice-onto-raw) instead of whole-record ApplyEntity replace
- [x] Push path: sync client creates/updates/deletes via `/api/v1`; delete
`handleSyncPut`
- [x] Temp-id reconciliation: provisional local id → push create without id →
adopt remote id on ack → rename local doc → remap references
- [x] Relation-id ordering: push entities first, adopt ids, then push relations
with `from`/`to` resolved to remote ids
- [x] Keep the manifest; confirm the feed row-gate is unchanged

## Research

- [x] ~~/research survey~~ (N/A: approach is settled via the design trail;
no unfamiliar subsystem — reuses existing `/api/v1` + ApplyEntity)

## Security

- [x] Confirm read leak closes: sync fetch inherits `visible:` redaction via the
single `/api/v1` read path
- [x] Confirm write leak closes: push goes through `validateFieldWrite` like the
SPA
- [x] Confirm NO new SPA write-core risk: `/api/v1` create/update/delete get no
id mode, no suppression flag
- [x] Confirm pull-apply splice can't erase local hidden fields (data-destruction
bug does not recur)
- [x] Confirm the manifest feed stays row-level-only (no field decision in feed)

## Test Plan

- [x] Read: sync fetch of a `visible:`-redacted entity returns granted fields
only (now an `/api/v1` GET test / reuse existing coverage)
- [x] Pull-apply: redacted fetch spliced onto a local record with hidden fields →
hidden fields survive locally
- [x] Push: field-write ACL rejects an out-of-bounds field on push (parity with
SPA `validateFieldWrite`)
- [x] Push create: temp-id → remote-id rename; local doc renamed; references
remapped
- [x] Relation ordering: local `TEMP→B` relation pushes with resolved remote
`from`
- [x] Regression: manifest feed row-gate unchanged (cannot-read omitted;
partial-prop still appears)

## Risk Assessment

- [x] Client rewrite is the bulk — temp-id reconciliation + reference remap is the
trickiest part; call out ordering + failure/rollback (push-ack lost)
- [x] N+1 fetch tradeoff acknowledged (parallelizable; batch-GET deferred)
- [x] Migration/compat: are there deployed replicas on the old `/api/sync/`
record endpoints? (version skew during rollout)

## Design Review

- [x] Run `/design-review` on THIS (fancy-browser) shape before implementation
- [x] All critical/significant findings addressed in plan

**Design-review outcome (2026-08-15):** 8 findings. Three "critical" findings
dissolved under user corrections + the `_redacted` wire field (DEC-T0XIWQ):
- **redacted-vs-deleted (was critical):** RESOLVED by `_redacted` — pull applies
as a PATCH; present→patch, named-in-`_redacted`→leave (hidden, not deleted),
in-neither→unset. No feed tombstones, no leak. This is the crux and it holds.
- **two incompatible ETags (was critical):** RESOLVED — replica uses the REMOTE''s
ETag as its baseline (no local canonical recompute).
- **"SPA can''t read relations" (was critical):** downgraded — the SPA CAN read
relations; the real work is moving the client relation DTO to the v1 shape and
exposing a v1 relation read with body + ETag (RR-SYNCR1).

Surviving work, recorded as review-responses:
- **RR-SYNCR1** (significant, open) — v1 relation read shape + client DTO.
- **RR-SYNCR2** (significant, open) — push ordering: defer relations on un-adopted
temp ids; two-temp-endpoint remap; lineage-fork guard.
- **RR-SYNCR3** (significant, open) — mirror delete only on manifest `Deleted:true`,
never infer from a bare GET 404.
- **RR-SYNCR4** (minor, open) — temp-id lost-ack: replica-side
reconcile-before-repush; NO permanent server dedup storage; rare double-create
is an accepted, manually-correctable residual (user ruling).

Cleared as non-issues: push field-write leak IS fixed by routing through the v1
`validateFieldWrite`; keeping `ApplyEntity`-style suppression for the local
pull-apply is correct. SPA `/api/v1` write CORE stays untouched (no id mode, no
suppression flag) — the only v1 additions are a relation read shape and possibly
a transient create-reconcile assist, neither on the shared write core.
