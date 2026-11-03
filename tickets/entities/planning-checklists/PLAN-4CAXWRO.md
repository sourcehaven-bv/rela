---
id: PLAN-4CAXWRO
type: planning-checklist
title: 'Planning: Verified fake API and property tests for SPA editing'
started: "2026-10-29"
completed: "2026-10-29"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem.** Switching items in the SPA and then editing can edit the previous
item. One likely cause, found while planning: `EntityDetail.loadView`
(`EntityDetail.vue:1089`) assigns `viewData` without checking that the route
still names the entity it fetched. `refreshView` (line 1043) has that guard;
`loadView` and `sectionReorder.reload` (line 262) do not.
`EntityDetailPanel.vue:66` mounts `EntityDetail` without a `:key`, so the list
and kanban side panel reuse one instance across rows. Two failure orders follow:

1. Switch A to B; A's slower response lands last. The panel shows A under
B's route; Edit and autosave target A.
2. Switch A to B and press Edit before B's view lands. The previous entry is
still on screen by design ("keep previous content"), so `editEntity` (which uses
`servedRef`) opens A's form.

Case 2 is arguably correct (the user edits what is on screen) but surprising;
the property test decides which invariant we hold, see Approach.

**Scope.**

In:
- A TypeScript fake of the data-entry API, in-process, as an axios adapter.
Covers: static schema and config snapshot; entity get, list, create, update,
delete; `_views/<type>/<id>` for a view with one editable entry section (entry
only); per-field `_versions` and 412 with `conflicts`/`versions`;
`entity:changed` SSE events; `_actions` as the server returns it for a project
without `acl.yaml`.
- An inert-route table: typed empty answers for the routes EntityDetail and
DynamicForm call but these tests do not exercise (commands, scope navigation,
comments, presence, live subscriptions, transforms, actions, templates). Each
entry names the route and the empty value.
- A differential test of the fake against `rela-server -tags memorybackend`.
- One model-based fast-check test for `EntityDetail` row switching and
editing, and one for `DynamicForm`.
- The fix for the bug the tests find.

Out:
- ACL, automations, validation rules, `_actions` computation, relations,
view sections, comments, channels, attachments. A route that is in neither the
implemented nor the inert table answers 501 with the route in the body.

**Acceptance Criteria:**
1. The differential test passes on the fake and fails on each of three
mutated fakes: no 412 on a stale token, token not changed on write, `_versions`
missing from the PATCH response.
2. The `EntityDetail` property test fails on current code with a shrunk
sequence that names the order (switch, late response, edit), without the order
being written into the test.
3. Both property tests pass after the fix.
4. Every property test is deterministic for a seed, logs the seed on
failure, and runs 100 cases in under 30 s in CI.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: approach chosen in discussion with the user: verified fake over real server or plain fake)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (see above)

**Existing Solutions:**
- `fast-check` 4.x is already a dev dependency (`frontend/package.json`).
`fc.commands` + `fc.modelRun`/`fc.asyncModelRun` give model-based testing;
`fc.scheduler()` controls promise resolution order. Chosen.
- MSW (Mock Service Worker) was considered as the fake's transport. Rejected:
the unit suite already replaces `axios.defaults.adapter`
(`src/test/setup.ts:197`), so an adapter is the smallest seam and keeps
`noNetwork.test.ts` meaningful.
- Prior art in the repo: `storetest` conformance harness (memstore is a
verified fake of pgstore); `storetest.RunGraphDifferential` (randomized
differential against a reference); `frontend/stress/fuzzRunner.ts` (fast-check
over UI actions, liveness oracles only); `frontend/stress/serverProcess.ts`
(spawns `rela-server` on a temp project).
- Reference: "verified fakes" (contract-tested test doubles) as used for
cloud SDK emulators; same idea as the store harness.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. *Fake* (`frontend/src/test/fakeApi/`). A `FakeServer` class holds entities
in a Map keyed by address, a version counter per field, and an event listener
list. `fakeAdapter(server, scheduler?)` returns an axios adapter. Without a
scheduler it answers synchronously. With one it schedules twice: before the fake
executes the request, and before it delivers the response. So fast-check picks
both the order in which the server runs concurrent writes and the order in which
the client sees answers. Each request is recorded (method, url, body, the time
step it was sent). Unknown routes answer 501 with the route in the body. Tokens:
opaque strings derived from a per-field counter, so equality is the only valid
comparison (as the API reference requires). SSE: tests install a fake
`EventSource` in place of setup.ts's stub that the `FakeServer` pushes
`entity:changed` frames to.
2. *Differential harness* (`frontend/contract/`, own
`vitest.contract.config.ts`, real node adapter, no `setup.ts` stub). The fixture
project is committed under `frontend/contract/fixture/` (one entity type, three
properties, a form, a view with one editable entry section, no `acl.yaml`), and
so is the fake's schema and config snapshot
(`frontend/contract/fixture/snapshot.json`), which the unit suite reads.
`globalSetup` builds `rela-server` with `-tags memorybackend`, copies the
fixture to a temp dir and starts the server on an ephemeral port. The first
contract test asserts the live schema and config responses equal the committed
snapshot; `npm run contract -- --update` rewrites it. The test generates command
sequences (create, get, patch with fresh or stale tokens from two clients,
delete, list) and runs each through `src/api` functions against both. It
compares status, body (normalized: ids mapped by creation order, timestamps
dropped, tokens compared by equality pattern rather than value, `_actions`
included) and SSE frames received. SSE is read with `fetch` streaming and a
small frame parser, so no EventSource dependency. Isolation: each run uses fresh
entities, so the server is started once per file.
3. *Property tests* (`EntityDetail.switching.prop.test.ts`,
`DynamicForm.switching.prop.test.ts`). Commands: `select(id)` (changes the
panel's entity prop), `deliver()` (lets the scheduler resolve one pending
response), `editContent`/`editField(value)`, `pressEdit()`, `advance(ms)` (fake
timers for debounces only, `toFake` limited to
`setTimeout`/`clearTimeout`/`Date`), `externalChange(id)` (another client
writes, SSE fires). Mount with a real `createRouter` + `createMemoryHistory`,
not the mocked router of the existing tests. "On screen" is read from the
rendered DOM (the entry id element), never from component refs. Model: the
selected id and the server's entities. Invariants checked after every command:
   - every PATCH targets the entity whose data was on screen when the edit
was made;
   - once all responses are delivered, the entry on screen is the selected
entity's;
   - `pressEdit` opens the form for the entity on screen (decision below);
   - after settling, the fake server holds what the model predicts.
4. *Fix*: a load sequence guard in `loadView` and `sectionReorder.reload`
(same shape as `refreshView`), and the Edit decision from point 3.
5. *CI*: the property tests run in the normal `npm run test:run`. The
contract test gets a `just contract-fake` recipe and a CI job, path-filtered on
`internal/**`, `cmd/rela-server/**`, `frontend/src/api/**`,
`frontend/src/test/fakeApi/**`, `frontend/contract/**`. It reuses the CI Go
build.

**Decision: Edit during a switch.** While B loads, A stays on screen by design.
Edit opens the form for what is on screen (A), with no extra rule. The bug is a
stale entry that stays after B's response has arrived, which the sequence guard
fixes. If the user prefers otherwise, only this invariant changes.

**Design review:** the RR findings on TKT-7LTLHW3 are folded in above.

**Alternatives rejected:** real server for SPA tests (user chose the fake for
speed and simple timing control); a hand-written fake without the differential
test (drifts silently); periodic instead of per-change differential runs (drift
goes unnoticed between runs).

**Dependencies:** fast-check (present), axios adapter API, vitest `globalSetup`,
Go toolchain in the contract CI job.

**Files to modify:**
- new `frontend/src/test/fakeApi/{server,adapter,eventSource,index}.ts`
- new `frontend/contract/{globalSetup,fakeVsServer.contract.test}.ts`
- new `frontend/contract/fixture/` (project files, `snapshot.json`)
- new `frontend/vitest.contract.config.ts`, `package.json` script
- new `frontend/src/components/entity/EntityDetail.switching.prop.test.ts`
- new `frontend/src/components/forms/DynamicForm.switching.prop.test.ts`
- `frontend/src/components/entity/EntityDetail.vue` (fix)
- `justfile`, `.github/workflows/ci.yml` (contract job)
- `frontend/CLAUDE.md` (testing section: when to use the fake)

## Security Considerations

- [x] ~~Input sources identified (user input, config, external APIs)~~ (N/A: test-only code; the fix narrows which response may land and adds no input)
- [x] ~~Input validation approach defined (allowlist preferred over blocklist)~~ (N/A: test-only code)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] ~~Error handling doesn't leak sensitive information~~ (N/A: test-only code)

**Input Sources & Validation:** N/A, test code.

**Security-Sensitive Operations:** The fake has no ACL, so a test that passes
against it says nothing about authorization. The fake's module doc and
`frontend/CLAUDE.md` state this, and the 501 default stops it from quietly
answering for gated routes. The fix itself is security-positive: a late response
for an entity the user left can no longer replace the one on screen.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- AC1: contract test on the real fake; three mutated fakes behind a test
flag, each expected to fail the contract.
- AC2: run the EntityDetail property test on current `develop`; record the
shrunk counter-example in the ticket.
- AC3: same tests after the fix.
- AC4: CI timing; seed printed by fast-check on failure.

**Edge Cases:** switch back to the same entity (A, B, A) while A's first
response is pending; delete of the selected entity; 412 during a switch; SSE
refresh deferred by a busy autosave (`refreshDeferred`); a 412 with an empty
`conflicts` (resend).

**Negative cases:** fake routes outside scope answer 501; a request with a
precondition on a field it does not write answers 400 `invalid_precondition` on
both sides.
