---
id: PLAN-H6XW20
type: planning-checklist
title: 'Planning: Autosave conflict resolution: send If-Match, three-way merge on 412, bounded auto-retry'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

Re-planned 2026-08-28 around **per-field version preconditions** instead of a
whole-entity ETag `If-Match`. The design rationale, wire shape, cost analysis
and merge-domain specification live on TKT-2VDVHF; this checklist plans the
work.

IN scope:

- **Server**: per-field version tokens (`_versions` on the GET/PATCH response
  body), an optional `preconditions` field on PATCH, and a 412 body naming the
  conflicting fields plus current tokens.
- **Client**: capture `_versions` from the response body; autosave sends
  preconditions for exactly the keys it writes; explicit `DynamicForm` save
  narrowed to a dirty-field delta before it preconditions.
- On 412: merge the named fields → re-PATCH with fresh tokens; bounded
  auto-retry (3 attempts, jittered backoff).
- Markdown body merged with a real diff3 library, on any 412 including autosave
  (user decision: "Both, same logic").
- Conflict UI only after the retry bound is exhausted or a genuine conflict is
  detected.
- Fix the stale second clause of the `useAutoSave.ts:18-20` comment, preserving
  the (true, load-bearing) FIFO clause.
- **Delete** `dirtyFormRegistry` (decided — see Design Review below).

NOT in scope:

- CRDT / concurrent collaborative editing (user: later possibility).
- Making preconditions mandatory server-side (breaks MCP/CLI/Lua; 412s disjoint
  edits the design already handles).
- Stored per-field version counters — derived hashes first.
- Reviving SSE-into-open-form (payload is type-only by security design,
  TKT-POT9GQ).
- Temporary edit lock (documented as the weighed alternative).
- Migrating `DynamicForm` to the Pinia Colada query layer (FEAT-XY2D1L) — **not
  a blocker**.
- The `DynamicForm` merge base and the dead `EntityCache.etag`: split out to
  **TKT-52OFC9** and already landed.

**Acceptance Criteria:**

1. **AC1 — versions exposed:** a GET returns `_versions` covering every
   non-redacted property plus `content` and `relations`. A redacted property has
   NO entry, asserted directly.
2. **AC2 — preconditions sent:** autosave property/content/relations PATCHes
   send preconditions for exactly the keys in their own body — no more (a key
   not being written is a 400) and no fewer.
3. **AC3 — disjoint fields auto-resolve with NO 412 at all:** Alice sets
   `status`, Bob sets `owner` concurrently → Bob's PATCH **succeeds first time**.
   Stronger than the old AC3, which merely required the 412 to be absorbed
   invisibly; under per-field preconditions the 412 never happens.
4. **AC4 — same-field conflict surfaces:** both edit `description` → 412 names
   `description` → merge finds ours/theirs/base all differ → conflict surfaced,
   neither value silently discarded.
5. **AC5 — body disjoint hunks merge:** Alice edits para 2, Bob edits para 7 →
   diff3 merges both, no conflict, no markers.
6. **AC6 — no markers, enforced as control flow:** a merge result with ANY
   conflict entry MUST NOT be written. Tests assert **no PATCH is issued** on a
   conflicting body merge, in addition to asserting the output contains no
   `<<<<<<<` / `=======` / `>>>>>>>`.
7. **AC7 — retry bounded:** at most 3 attempts; a perpetually-contended entity
   terminates with a conflict rather than looping. No unbounded retry, no
   livelock.
8. **AC8 — no behavioural regression, with tightened etag assertions.**
   (Rewritten per RR-GDE3PY — the previous "suite passes unchanged" was
   unachievable and pressured the implementer to loosen assertions.) The
   existing suites must continue to guarantee the same **behaviour**: FIFO
   ordering, debounce coalescing, no-op suppression, warning routing, and the S5
   `lastSeenServer` invariant. Where a test asserts on `store.update` call shape,
   it **must be updated to assert the precondition payload positively** — an
   assertion that silently keeps passing while ignoring the new field is a
   failure of this AC, not a pass. Precondition assertions are **tightened,
   never relaxed**.
9. **AC9 — hidden-field churn does not block writes:** a concurrent write to a
   property the client never sends (e.g. a Lua task writing `salary`) does NOT
   cause a 412 on a `title` write. Regression test for RR-X52UBP.
10. **AC10 — a redacted property is never unset:** a merge over an entity with a
    redacted property never emits `properties_unset` for it. Regression test for
    RR-R2A2T5.
11. **AC11 — cross-channel writes do not collide:** an interleaved content edit
    and property edit within one FIFO chain produce no 412. Regression test for
    RR-DBL90Y.

## Research

- [x] ~~run `/research`~~ (N/A: approach settled with user + a dedicated read-only investigation, findings recorded on the ticket)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (investigation findings are on TKT-2VDVHF with file:line
evidence)

**Existing Solutions:**

- **diff3 libraries** (no diff dependency exists in `frontend/package.json`
  today — this is a NEW dep): `node-diff3` (small, pure diff3 with
  `mergeDiff3` / `merge` returning conflict regions), `diff` (jsdiff — larger,
  has `diff3Merge`), `diff3`. Selection criteria: bundle size (SPA ~372KB and
  deliberately cached), returns structured conflict regions rather than
  marker-embedded text (**required** — AC6), ESM/tree-shakeable, maintained.
  Recommend `node-diff3` pending a size check. Note `mergeDiff3` embeds markers
  **by default** — the integration must consume its structured regions, which is
  exactly the trap RR-U68IVA names.
- **Merge base already exists**: `lastSeenServer` / `lastSeenContent`
  (`useAutoSave.ts:165-167`), written ONLY from server responses per the S5
  invariant. **Correction to the original plan**: it was seeded on
  `SectionEditForm` and `EntityDetail` but NOT on `DynamicForm` — fixed in
  TKT-52OFC9. The `baseRecorded` sentinel below covers any future surface that
  forgets.
- **Per-field tokens are the existing fold, uncollapsed.** `computeEntityETag`
  (`api_v1.go:2001-2034`) already sorts property keys and writes `k=%v;` per
  key. Emitting a token per key is the same work, hashed per key rather than
  accumulated — not a new mechanism. **This is the key discovery that made the
  re-plan cheap.**
- **The GET path already fetches relations twice** (`api_v1.go:764` for
  serialization, again inside `computeEntityETag` at `:772`). Threading the
  fetched slice through removes a redundant store query, so the version work
  makes this path cheaper.
- **PATCH is already field-granular** (`write_handler.go:386-390`) with a
  server-side read-modify-write (`maps.Copy`, `:395`/`:411`/`:445-450`) — absent
  keys survive. The wire protocol was always sparse; only the precondition was
  whole-entity.
- **Test harness to extend**: `useAutoSave.test.ts` mocks at the Pinia store
  seam with fake timers driving the debounce chain; `mergeServerResponse` is
  already directly unit-tested. A pure merge function tests the same way with no
  new infrastructure.
- **`fast-check` is available** (devDep) but used ONLY by the out-of-suite
  stress harness, never under `src/**`. A property test for a pure merge
  function would be the first such usage — viable, flag as a new pattern.

**External prior art:** git's diff3 (the model the user cited); CouchDB/PouchDB
conditional-PUT + client-side merge; the HTTP optimistic-concurrency pattern
(RFC 9110 §13.1.1). Per-field preconditions specifically mirror **conditional
requests scoped to a sub-resource** — the same reasoning that makes PATCH sparse
in the first place.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

**Step 1 — server: per-field version tokens.** Extract the per-key fold that
`computeEntityETag` already performs into a helper that returns a token per
property plus one for content and one for the relation set. Token =
first 4 bytes of `sha256(entityID | type | fieldKey | value)`, hex — the key is
part of the material so two fields with the same value get different tokens.
Emit as `_versions` alongside `_fields` / `_relations` in
`entitySerializer.forWire`, built from the **post-redaction** property map so a
hidden key has no entry. Thread the already-fetched `outgoingRelations` slice in
rather than re-querying. `computeEntityETag` itself is untouched — the
whole-entity ETag remains the HTTP-caching validator.

**Step 2 — server: `preconditions` on PATCH.** Decode alongside the existing
fields, check where the `If-Match` check lives now (`write_handler.go:375-384`)
against the same ungated `h.reader.getEntity` seam, before any mutation. A
precondition naming a key the body does not write is a **400** (not ignored —
see the ticket for why that would be an oracle). On mismatch, 412 with a
`meta.conflicts` map of `{expected, actual}` per losing field plus a `versions`
block carrying current tokens, so the retry needs no GET.

**Step 3 — client: capture `_versions`.** It rides in the JSON body, so
`client.ts`'s `.data` unwrap is left alone. **The original plan's Step 0 (an
axios metadata envelope) is deleted** — it was only needed because the ETag
lived in a header. This is the single largest scope reduction from the re-plan.

**Step 4 — client: send preconditions.** Each autosave channel builds its
precondition set from its own body, so the three cannot collide. The retained
versions map is a **single mutable ref updated inside the then-handler** of
every PATCH response — never captured at enqueue time (RR-DBL90Y). Narrow the
explicit `DynamicForm` save to a dirty-field delta first (RR-U3ZF9A).

**Step 5 — pure merge function.** `frontend/src/composables/threeWayMerge.ts`:
a pure `(base, ours, theirs) → {merged, conflicts[]}`, unit- and
property-testable without timer/Pinia machinery. Domain = the keys named in the
412 `conflicts` block. Omitted key ⇒ UNCHANGED, never deleted. **Never** emit
`properties_unset` from theirs-absence. Body merge delegates to the diff3
library, mapping its conflict regions into the same `conflicts[]` shape —
never emitting marker text. Guarded by a `baseRecorded` sentinel: with no
recorded base the merge **refuses to run** and falls back to current behaviour.

**Step 6 — retry loop.** On 412: merge the named fields → re-PATCH with the
tokens from the error body. Max 3 attempts, jittered backoff. **Control-flow
invariant: any conflict entry ⇒ no write** (AC6/RR-U68IVA). Exhausted or
conflicting → surface via the existing error channels (`contentError`,
`relationWarnings`, `onError`) rather than a new modal, if that surface fits.

**Step 7 — delete `dirtyFormRegistry`.** Remove `dirtyFormRegistry.ts`, its
test, and the `DynamicForm` registration. (Decision recorded under Design
Review.)

**Step 8 — fix the comment** at `useAutoSave.ts:18-20`: keep the FIFO clause,
replace the false SSE clause with the precondition strategy.

**Files to modify:**

- `internal/dataentry/api_v1.go` — extract the per-key fold; emit `_versions`
- `internal/apiwire/v1/responses.go` — `_versions` field on the wire entity, beside `_fields` (`responses.go:32`)
- `internal/dataentry/entityserializer.go` — populate `_versions` in `forWire`, AFTER `stripHiddenProperties` runs (`entityserializer.go:105-120`), which is what makes the post-redaction guarantee structural
- `internal/dataentry/write_handler.go` — decode + check `preconditions`; 412 body
- `frontend/src/types/entity.ts` — `_versions` on the entity type
- `frontend/src/api/entities.ts`, `frontend/src/stores/entities.ts` — pass preconditions; version-bearing read bypasses the TTL
- `frontend/src/composables/threeWayMerge.ts` (new) + `.test.ts`
- `frontend/src/composables/useAutoSave.ts` — preconditions, 412 retry, comment fix
- `frontend/src/components/forms/DynamicForm.vue` — dirty-delta explicit save; drop the registry call
- `frontend/src/components/forms/dirtyFormRegistry.ts` + `.test.ts` — **deleted**
- `frontend/package.json` — diff3 dependency
- `docs/data-entry/api-reference.md` — document `_versions` + `preconditions`
- Tests: extend `useAutoSave.test.ts`; new merge tests; Go handler tests

**Alternatives considered:** whole-entity `If-Match` (the original plan —
rejected: it is the root cause of five review findings, because the ETag is
whole-entity while the wire protocol is a sparse PATCH); mandatory server-side
preconditions (rejected — breaks other clients); stored version counters
(deferred — derived hashes need no migration); temp edit lock (deferred,
documented on the ticket); CRDT (out of scope per user).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- **`theirs` comes from the server through the normal ACL-gated path** — the
  merge never sees data the user could not already read. No new read surface.
- **`_versions` is built from the post-redaction property map.** A hidden
  property has no token, so a client cannot learn that it exists or that it
  changed. This is the difference from the whole-entity ETag, which changed
  whenever a hidden field changed and thereby leaked a change-detection signal
  (and, per RR-X52UBP, made the entity unwritable). Asserted by AC1.
- **Preconditions are restricted to keys being written**, enforced as a 400.
  This is the structural guarantee behind RR-R2A2T5: a redacted field cannot
  enter the precondition set, cannot enter the merge domain, and cannot be
  unset. Accepting a precondition on a non-written key would also make the
  endpoint a change-detection oracle for data the caller cannot read.
- **`properties_unset` is never derived from absence** — only from an explicit
  local UNSET sentinel. Absence on the wire means "redacted or never set", never
  "delete this".
- **New dependency (diff3 library)** is supply-chain surface on a
  security-reviewed frontend: pin the version, prefer small and
  dependency-free, review the transitive tree.

**Security-Sensitive Operations:**

- No new endpoints, no auth changes, no crypto. Tokens are non-secret
  integrity/versioning values derived from data the caller already received;
  they are truncated hashes, not capabilities — possessing one grants nothing.
- Token collisions (4-byte truncation) are a **liveness** concern, not a
  security one: a collision means a genuine change is not detected, degrading to
  today's last-write-wins for that field. Widen the token if measurement ever
  justifies it.
- Conflict UI only ever renders values already returned by the viewer's own
  ACL-gated read.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- **AC1**: Go handler test — GET returns `_versions` for every non-redacted
  property plus `content`/`relations`; a redacted property has NO entry; the
  token changes when (and only when) that field changes.
- **AC2**: store-level tests asserting preconditions are sent for exactly the
  written keys; a precondition on an unwritten key is a 400 (Go test).
- **AC3/AC9/AC11**: `useAutoSave.test.ts` — mock `store.update`; assert the
  **absence** of a 412 for disjoint-field, hidden-field-churn and cross-channel
  scenarios. These are the three findings the design dissolves, so they are
  pinned as regression tests rather than left as reasoning.
- **AC4**: mock a 412 naming one field, then success; assert refetch-free merge
  → re-PATCH (AC4 conflict path surfaces instead when all three differ).
- **AC5/AC6**: pure-function tests on `threeWayMerge` with multi-paragraph
  bodies. Assert merged output has no markers **and** that no PATCH is issued
  when `conflicts[]` is non-empty (control-flow assertion, RR-U68IVA).
- **AC7**: mock a permanently-412ing store; assert exactly 3 attempts then a
  terminal conflict, no infinite loop (fake timers make this deterministic).
- **AC8**: full existing `useAutoSave.test.ts` + `SectionEditForm.test.ts`
  behavioural guarantees hold; call-shape assertions updated to assert the
  precondition payload **positively**.
- **AC10**: merge over an entity with a redacted property never emits
  `properties_unset` for it.
- **Property-based** (`fast-check`, first use under `src/**`): for random
  (base, ours, theirs) assert merge is deterministic; `ours===theirs` ⇒ result
  equals both; `ours===base` ⇒ result equals theirs; result never contains
  conflict markers; no input value is dropped without appearing in
  `conflicts[]`.
- **Integration**: an E2E two-session concurrent-edit scenario is the only true
  end-to-end proof (`e2e/tests/`). May be follow-up if the harness cannot drive
  two authenticated sessions.

**Edge Cases:**

- **No versions known** (first load, cache miss) → send no preconditions →
  current behaviour. Must not break.
- **No base recorded** → `baseRecorded` sentinel false → merge refuses to run,
  falls back to current behaviour. `undefined` is ambiguous between "never seen
  the server" and "genuinely absent server-side"; the sentinel disambiguates.
- **412 on the retry itself** with newer tokens → counts against the bound.
- **Entity deleted between attempt and refetch** → 404 → surface as deleted, not
  as a merge conflict.
- **Automation-managed fields** (`updated_at`, `{{today}}`, status transitions —
  automations mutate the entity before persisting): never preconditioned because
  never written by the client, so they are structurally incapable of
  conflicting. Explicitly tested.
- **Property genuinely unset server-side by automation** vs. locally edited:
  handled by the UNSET sentinel path only — never inferred from absence.
- **Relations**: one token for the whole edge set. An incoming-edge or
  edge-property change does not invalidate it (outgoing-only, as the ETag is
  today) — document, don't fix here.
- Empty/whitespace body; CRLF vs LF (line-based diff sensitivity); very large
  body (diff3 perf).
- Token collision → missed detection, degrades to today's behaviour.

**Negative Tests:**

- Malformed/absent `_versions` → no preconditions, no crash.
- Precondition on a key not in the body → 400 with a clear message.
- Merge library throwing on pathological input → degrade to a surfaced
  conflict, never a silent overwrite or a lost patch.
- Retry exhausted → user-visible conflict, pending edits NOT discarded.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- **Spurious 412s — largely designed out (was the highest risk).** Under the
  original whole-entity ETag, any concurrent change 412'd an unrelated save, and
  three findings (RR-X52UBP, RR-DBL90Y, RR-PP9UEF) were consequences. Per-field
  preconditions mean a 412 only for a genuinely contested field. AC3/AC9/AC11
  pin this as tests rather than hope. Residual risk: two users really editing
  the same field, which is the case that *should* conflict.
- **Silent-overwrite regression.** A merge bug could discard a value with no
  conflict raised — the exact failure this ticket exists to fix. Mitigation:
  pure function + property tests asserting no input is dropped without a
  conflict entry; the control-flow rule (any conflict ⇒ no write).
- **Server surface grows.** Unlike the original plan this touches the API. Cost
  is contained: both fields are additive and optional, no versioning needed, and
  no storage schema change (derived hashes). The compensating saving is that the
  client-side axios metadata-envelope refactor is no longer needed at all.
- **Autosave is load-bearing and heavily reviewed** (TKT-E6094 + design review +
  a large test suite). Touching FIFO/debounce risks regressions well beyond
  conflicts. Mitigation: merge logic lives OUTSIDE the composable as a pure
  function; AC8 pins behavioural guarantees.
- **New frontend dependency** — bundle size and supply chain. Mitigation: prefer
  small/dependency-free, pin, check size delta.
- **Token collisions** at 4-byte truncation degrade to today's behaviour for
  that field. Widen if measured.
- **Mid-typing merge semantics** (user chose "both, same logic"): merging a
  partial sentence is correct at hunk granularity but may surprise. Mitigation:
  disjoint hunks only; overlapping always conflicts.

Effort: **l** — larger than the original m/l. The client side got *smaller*
(no axios envelope refactor, no ETag plumbing), but the design now includes
server work: per-field tokens, the `preconditions` field, and a structured 412
body.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist deferred to implementation (created on the `in-progress`
      transition by the standard automation, per the documented ticket workflow)

**Documentation Impact:**

- [x] `docs/data-entry/api-reference.md` — `_versions` response field,
      `preconditions` request field, the 412 body shape
- [x] `docs/data-entry.md` — concurrent-edit behaviour: what auto-merges, what
      surfaces a conflict, what the user should do
- [x] `frontend/CLAUDE.md` — the merge/retry pattern and the rule that the
      version-bearing read path must bypass the TTL cache
- [x] Fix the stale `useAutoSave.ts:18-20` comment (tracked as AC-adjacent work)
- [x] ~~docs/cli-reference.md~~ (N/A: no CLI surface change)
- [x] ~~docs/metamodel.md~~ (N/A: no metamodel change)

## Design Review

- [x] `/design-review` run before implementation — 10 findings recorded as
      RR-* entities linked via `has-review-response`
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 10 findings (4 critical, 4 significant, 2 minor),
all re-verified against develop on 2026-08-28 and all still reproducing at the
time of re-planning. Resolution:

**Dissolved by the per-field design** (the whole-entity ETag was their shared
root cause):

- **RR-X52UBP** (critical) — hidden-field churn no longer 412s: we never
  precondition on a field we do not write. AC9.
- **RR-R2A2T5** (critical) — a redacted field cannot enter the precondition set
  or the merge domain; `properties_unset` is never derived from absence. The
  plan's old edge case, which had this backwards, is corrected. AC10.
- **RR-DBL90Y** (significant) — disjoint precondition sets cannot collide; the
  versions ref is updated inside the then-handler, never captured at enqueue.
  The FIFO clause of the `useAutoSave` comment is preserved as correct. AC11.
- **RR-PP9UEF** (critical) — the 412 body carries current tokens, so the retry
  needs no refetch; where a refetch is needed it bypasses the TTL. Long-term
  dissolved by FEAT-XY2D1L.

**Folded into the plan as required work:**

- **RR-VQQQ60** (critical) — `DynamicForm` merge base: shipped in TKT-52OFC9;
  the `baseRecorded` sentinel is retained here (Step 5) so the merge refuses to
  run without a base.
- **RR-P6ZFSV** (significant) — merge domain is now specified explicitly: the
  keys named in the 412, an omitted key means UNCHANGED, automation-managed
  fields cannot conflict.
- **RR-U3ZF9A** (significant) — the explicit `DynamicForm` save is narrowed to a
  dirty-field delta before it preconditions (Step 4).
- **RR-U68IVA** (minor) — the marker prohibition is stated as control flow ("any
  conflict entry ⇒ no write") and asserted as "no PATCH issued", not only as an
  output property. AC6.
- **RR-GDE3PY** (minor) — AC8 rewritten: behavioural guarantees hold, call-shape
  assertions are updated to assert the precondition payload positively, and
  assertions are tightened rather than relaxed.

**Decided against the original recommendation:**

- **RR-QSO6HF** (significant) — `dirtyFormRegistry` is **deleted**, not wired.
  Its `anyFormDirty` union across all forms registered for an entity is wrong
  for merge arbitration: a dirty side panel would cause the main form's stale
  value to be preserved over the server's. The composable's local `isDirty` is
  precise and already consulted. `anyFormDirty` has zero production consumers
  and the SSE-refetch path it was built for does not exist. Step 7.
