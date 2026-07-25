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

IN scope:

- Client-layer ETag capture (`getWithMeta`-style typed envelope), adopted on entity read/write paths only.
- Autosave + explicit-save paths send `If-Match`.
- On 412: refetch → three-way merge (properties, content, relations) → re-PATCH with the fresh ETag; bounded auto-retry 3 attempts with jittered backoff.
- Markdown body merged with a real diff3 library, **on any 412 including autosave** (user decision: "Both, same logic").
- Conflict UI only after the retry bound is exhausted or an overlapping-hunk / same-field conflict is detected.
- Fix the stale `useAutoSave.ts:18-20` comment (user decision).
- Resolve `dirtyFormRegistry` — revive it for the merge flow or delete it (user decision: handle here, not separately).

NOT in scope:

- CRDT / concurrent collaborative editing (user: later possibility).
- Making `If-Match` mandatory server-side (breaks MCP/CLI/Lua; 412s disjoint edits the merge handles).
- Reviving SSE-into-open-form (payload is type-only by security design, TKT-POT9GQ).
- Temporary edit lock (documented as the weighed alternative; may be layered later for high-contention types).

**Acceptance Criteria:**

1. **AC1 — ETag retained:** a GET of an entity makes its ETag available to the store/form; a PATCH response updates it. Verified by unit test asserting the etag is captured from response headers (today `EntityCache.etag` is dead).
2. **AC2 — If-Match sent:** autosave property/content/relations PATCHes and the explicit `DynamicForm` save all send `If-Match` when an etag is known.
3. **AC3 — disjoint fields auto-resolve:** Alice sets `status`, Bob sets `owner` concurrently → Bob's 412 refetches, merges, re-PATCHes, both values present, **no UI shown**. (This is today's working behavior and must not regress into a user-visible conflict.)
4. **AC4 — same-field conflict surfaces:** both edit `description` to different values → after merge, `ours != base && theirs != base && ours != theirs` → conflict surfaced, neither value silently discarded.
5. **AC5 — body disjoint hunks merge:** Alice edits para 2, Bob edits para 7 → diff3 merges both, no conflict, no markers.
6. **AC6 — body overlapping hunks conflict:** both edit the same paragraph → conflict surfaced; **no `<<<<<<<`/`=======`/`>>>>>>>` markers ever written to the entity**.
7. **AC7 — retry bounded:** at most 3 attempts; a perpetually-contended entity terminates with a conflict rather than looping. No unbounded retry, no livelock.
8. **AC8 — no regression:** existing autosave suite (`useAutoSave.test.ts`, 512 lines) passes unchanged; FIFO chain, debounce, warning categorization, `lastSeenServer` S5 invariant all intact.

## Research

- [x] ~~run `/research`~~ (N/A: approach settled with user + a dedicated read-only investigation, findings recorded on the ticket)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (investigation findings are on TKT-2VDVHF with file:line
evidence)

**Existing Solutions:**

- **diff3 libraries** (no diff dependency exists in `frontend/package.json` today — this is a NEW dep): `node-diff3` (small, pure diff3 with `mergeDiff3`/`merge` returning conflict regions), `diff` (jsdiff — larger, has `diff3Merge`), `diff3`. Selection criteria for planning: bundle size (SPA is ~372KB and deliberately cached, `apps_handler.go:140-143`), returns structured conflict regions rather than marker-embedded text (**required** — see AC6), ESM/tree-shakeable, maintained. Recommend `node-diff3` pending size check.
- **Merge base already exists**: `lastSeenServer` / `lastSeenContent` (`useAutoSave.ts:164-168`), written ONLY from server responses per the S5 design-review invariant — exactly the three-way base needed. No new state required.
- **Server side already complete**: `computeEntityETag` (`api_v1.go:1805-1837`), `If-Match` enforcement (`write_handler.go:327-336`), fresh ETag on PATCH response (`:454-455`), GET ETag + `If-None-Match` 304 (`api_v1.go:767-774`). **No server changes needed.**
- **Test harness to extend**: `useAutoSave.test.ts` mocks at the Pinia store seam with fake timers driving the debounce chain; `mergeServerResponse` is already directly unit-tested (`:268`, `:409`). A pure merge function tests the same way with no new infrastructure.
- **`fast-check` is available** (devDep) but currently used ONLY by the out-of-suite stress harness (`stress/fuzzRunner.ts:13`), never under `src/**`. A property test for a pure merge function would be the first such usage — viable with the current vitest config, flag as a new pattern.

**External prior art:** git's diff3 (the model the user cited); CouchDB/PouchDB
conditional-PUT + client-side merge; the HTTP optimistic-concurrency pattern
(RFC 9110 §13.1.1).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

**Step 0 — ETag capture (prerequisite).** Add a metadata-returning method to the
client layer *alongside* the existing terse ones, e.g. `getWithMeta<T>(url):
Promise<{data: T; etag?: string}>` (and the PATCH equivalent). Rationale
(best-practice answer to the user's question): unwrapping every response to
`.data` is a common anti-pattern that makes ALL HTTP metadata unreachable — ETag
is just the first casualty (`Location`, `Retry-After`, pagination links are
next). A typed envelope keeps the axios coupling inside `client.ts` (nothing
outside touches `AxiosResponse`), keeps terse `get<T>` for the callers that
don't care, and gives the next piece of metadata one obvious home. **Adopt it
only on entity read/write paths in this ticket** — no big-bang migration of
existing call sites (consumer-side-interface rule favours narrow seams).
Populate the already-declared-but-dead `EntityCache.etag`.

**Step 1 — send `If-Match`.** Thread the retained etag through the existing
(already-present) parameter chain at the autosave call sites
(`useAutoSave.ts:318-320, 369-371, 415-417`) and the explicit save
(`DynamicForm.vue:887`). Kanban/list-bulk paths: include if cheap, else document
as follow-up.

**Step 2 — pure merge function.** A standalone, dependency-free module (e.g.
`frontend/src/composables/threeWayMerge.ts`) exporting a pure function: `(base,
ours, theirs) → {merged, conflicts[]}`. Pure so it is unit- and
property-testable without the sweep/timer/Pinia machinery. Per property:
`theirs===base` → ours; `ours===base` → theirs; equal → either; all differ →
conflict. Body: delegate to the diff3 library, mapping its conflict regions into
the same `conflicts[]` shape — **never** emitting marker text.

**Step 3 — retry loop in useAutoSave.** On 412: refetch (with meta, for the new
etag) → merge → re-PATCH. Max 3 attempts, jittered backoff.
Exhausted-or-conflicting → surface via the existing error/warning channels
(`contentError`, `relationWarnings`, `onError`) rather than a new modal, if the
existing surface fits.

**Step 4 — dirtyFormRegistry decision.** It was built exactly for "don't clobber
in-progress keystrokes" but `anyFormDirty` has zero consumers. Either wire it
into the merge flow (a dirty field is precisely what must not be overwritten by
a merge result) or delete it. **Recommend: wire it**, since the merge flow needs
that signal; decide finally in design review.

**Step 5 — fix the stale comment** at `useAutoSave.ts:18-20` describing a
non-existent SSE conflict strategy.

**Files to modify:**

- `frontend/src/api/client.ts` — metadata-returning method(s)
- `frontend/src/api/entities.ts`, `frontend/src/stores/entities.ts` — capture/store etag (populate the dead `EntityCache.etag`)
- `frontend/src/types/entity.ts` — etag carrier if it belongs on the entity-adjacent type
- `frontend/src/composables/threeWayMerge.ts` (new) + `.test.ts`
- `frontend/src/composables/useAutoSave.ts` — send If-Match, 412 retry loop, comment fix
- `frontend/src/components/forms/DynamicForm.vue` — explicit save sends If-Match
- `frontend/src/components/forms/dirtyFormRegistry.ts` — wire or delete
- `frontend/package.json` — diff3 dependency
- Tests: extend `useAutoSave.test.ts`; new merge tests

**Alternatives considered:** mandatory server-side `If-Match` (rejected — breaks
other clients, 412s disjoint edits); temp edit lock (deferred, documented on
ticket); CRDT (explicitly out of scope per user).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- **`theirs` comes from a server refetch through the normal ACL-gated GET** — the merge never sees data the user couldn't already read. No new read surface.
- **The refetch must go through the same ACL-gated endpoint**, never a bypass. A merge that pulled unredacted values and re-PATCHed them would write back fields the user cannot see — the same class of bug as the `internal/entitymanager/CLAUDE.md` "never redact a read that feeds a write" rule, inverted. **Design-review item:** confirm a field the user cannot see is never resurrected/clobbered by a merged PATCH.
- **New dependency (diff3 library)** is supply-chain surface on a security-reviewed frontend: pin the version, prefer a small dependency-free package, review its transitive tree.

**Security-Sensitive Operations:**

- No new endpoints, no auth changes, no crypto. The ETag is already emitted publicly on GET.
- Conflict UI must not render another user's rejected value in a way that implies authorization to see fields the viewer lacks — it only ever shows values already returned by the viewer's own ACL-gated GET.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- AC1/AC2: store-level tests asserting etag captured from response headers and forwarded as `If-Match` (extends the existing `stores/entities.test.ts:208-218` pattern, which today asserts pass-through of a literal only).
- AC3-AC4: `useAutoSave.test.ts` harness — mock `store.update` to reject with a 412 once, then resolve; assert refetch → merged patch → success with no error surfaced (AC3) vs. conflict surfaced (AC4).
- AC5-AC6: pure-function tests on `threeWayMerge` with multi-paragraph bodies; assert merged output and **assert the output contains no `<<<<<<<`/`=======`/`>>>>>>>` markers** (explicit regression guard for AC6).
- AC7: mock a permanently-412ing store; assert exactly 3 attempts then terminal conflict, no infinite loop (fake timers make this deterministic).
- AC8: full existing `useAutoSave.test.ts` + `SectionEditForm.test.ts` suites pass unchanged.
- **Property-based (new pattern, `fast-check` from `src/**` for the first time):** for random (base, ours, theirs), assert invariants — merge is deterministic; if `ours===theirs` result equals both; if `ours===base` result equals theirs; result never contains conflict markers; no input value is silently dropped without appearing in `conflicts[]`.
- **Integration:** an E2E two-session concurrent-edit scenario is the only true end-to-end proof (`e2e/tests/`). Scope in design review — may be follow-up if the harness can't drive two authenticated sessions.

**Edge Cases:**

- Etag unknown (first load, cache miss) → send no `If-Match` → current behavior (must not break).
- 412 on the retry itself with a *newer* etag → counts against the bound.
- Entity deleted between attempt and refetch → 404 on refetch → surface as deleted, not a merge conflict.
- Property deleted server-side (unset by automation) vs. locally edited → `theirs` absent, `ours` present: deletion-vs-edit is a genuine conflict, not "take ours". Explicitly test — `mergeServerResponse` already handles disappeared keys (`useAutoSave.ts:519-527`).
- Relations channel: ETag covers outgoing edges only, so an incoming-edge or edge-property change does NOT 412 (finding 4) — document, don't try to fix.
- Empty/whitespace body, body with CRLF vs LF (line-based diff sensitivity), very large body (perf of diff3).
- Concurrent 412s across the three channels (property/content/relations) racing within the FIFO chain.

**Negative Tests:**

- Malformed/absent ETag header → no `If-Match`, no crash.
- Merge library throwing on pathological input → must degrade to a surfaced conflict, never to a silent overwrite or a lost patch.
- Retry exhausted → user-visible conflict, pending edits NOT discarded.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- **Spurious-412 rate (highest risk).** Whole-entity ETag + per-property PATCH means any concurrent change 412s an unrelated save. If the merge+retry doesn't absorb these transparently, autosave becomes visibly janky — worse than today. Mitigation: AC3 pins the no-UI requirement; measure attempt counts in tests.
- **Silent-overwrite regression.** A merge bug could discard a value with no conflict raised — the exact failure this ticket exists to fix. Mitigation: pure function + property tests asserting no input is dropped without a conflict entry.
- **Autosave is load-bearing and heavily reviewed** (TKT-E6094 + design review + 512-line test suite). Touching its FIFO/debounce risks regressions well beyond conflicts. Mitigation: merge logic lives OUTSIDE the composable as a pure function; AC8 requires the existing suite green unchanged.
- **New frontend dependency** — bundle size and supply chain. Mitigation: prefer small/dependency-free, pin, check size delta.
- **Mid-typing merge semantics** (user chose "both, same logic"): merging a partial sentence is correct at hunk granularity but may surprise. Mitigation: disjoint hunks only; overlapping always conflicts. Revisit if it feels wrong in practice.

Effort: **m/l** — larger than first estimated, because ETag capture turned out
to be a prerequisite (nothing retains it today) rather than "just send the
header."

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [ ] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/data-entry.md` — concurrent-edit behavior: what auto-merges, what surfaces a conflict, what the user should do
- [x] `frontend/CLAUDE.md` — the merge/retry pattern + the client metadata-envelope convention, so the next agent doesn't re-add a bare `.data` unwrap on a path that needs headers
- [x] Fix the stale `useAutoSave.ts:18-20` comment (code doc, tracked as AC-adjacent work)
- [ ] ~~docs/cli-reference.md~~ (N/A)
- [ ] ~~docs/metamodel.md~~ (N/A)

## Design Review

- [ ] Run `/design-review` before starting implementation
- [ ] All critical/significant findings addressed in plan

**Design Review Findings:** (pending — key questions: (a) does a merged PATCH
risk writing back ACL-redacted fields? (b) diff3 library selection + bundle
impact, (c) revive-vs-delete `dirtyFormRegistry`, (d) retry bound/backoff
numbers, (e) is the existing error surface enough or is a conflict modal needed,
(f) E2E two-session test in-scope or follow-up)
