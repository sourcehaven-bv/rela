---
id: PLAN-T6ZQWF
type: planning-checklist
title: 'Planning: Surface list/view/dashboard command contexts in the SPA (only entity context renders today)'
status: pending
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN:
- `list` context → toolbar button on `EntityList.vue`, whole-list (no row selection)
- `global` context → button on `DashboardView.vue`
- `CommandModal` generalised from `entityId: string` to a context-shaped payload
- Fix `matchesPage`/`validateCommands` context-vs-`available_on` disagreement
- Docs update in `docs/data-entry.md`

OUT:
- **`view` context** — see the finding below; there is no `/view/:id` route to
hang a button on. Deferred rather than inventing a page.
- Sidebar entries for global commands (dashboard button only)
- The remote-file-delivery problem (that's TKT-PYPNWO)
- Wiring the unhandled `type:"open"`/`"entity"`/`"group"`/`"endgroup"` SSE events

**Key finding that reshaped the plan: there is no view page.**
`src/router/index.ts` has no `/view/:id` route. Views render *inside*
`EntityDetail` for a given entity, which is why `contextMatchesPage` maps
`entity` → matches `entity` OR `view`. So "add a button to the view page" has no
target. Adding a standalone view route is its own ticket, not a sub-task here.
Reduces the ticket from 3 surfaces to 2.

**Second finding: the exec path keys off `cmd.Context`, not page type.**
`handleCommandExec` (`commands.go:302`) switches on the command's *declared*
context to decide which query param it reads (`entity_id` / `list_id` /
`view_id`+`entity_id` / none). So the modal must send params matching
`cmd.context`, NOT the page it was clicked from. `Command.context` is already in
the served payload and typed in `types/config.ts:389` — no API change needed.

**Acceptance Criteria:**

1. A command with `context: list` + `available_on: {lists: [X]}` renders a
toolbar button on `/list/X`, and no button on an unrelated list.
2. Clicking it POSTs `/api/command/<id>?list_id=X` and streams SSE into the modal.
3. It renders with zero rows selected (proving independence from row `actions:`).
4. A command with `context: global` + `available_on: {dashboard: true}` renders a
button on `/dashboard` and POSTs with no entity/list param.
5. Existing entity-context behaviour is byte-identical (regression).
6. A config whose `context` and `available_on` disagree is rejected at
config-validation time with a clear message, instead of 404ing at exec time.

## Research

- [x] ~~Run `/research`~~ (N/A: approach is determined by existing structure; no
open design question worth a research doc)
- [x] Searched for existing libraries — N/A, this is internal UI wiring
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — small, structurally-determined change.

**Existing Solutions:**
- The entity surface is the reference implementation to mirror:
`EntityDetail.vue:321-324` (fetch), `:716-724` (render), `:333-335`
(runCommand), `:1185` (modal mount). The list/dashboard surfaces copy this
four-part shape.
- `getCommands` (`api/commands.ts:10`) already accepts `qualifier` and
`entityType` — the client API is list-capable today; only call sites are
missing.
- Backend needs no change for rendering: `commands_test.go:689,719,739` already
cover global/list/view resolution.
- `useListActions` is deliberately NOT reused — it is selection-scoped and
per-entity, the wrong shape (this is what the ticket is about).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified

**Technical Approach:**

1. **Generalise `CommandModal`.** Replace `defineProps<{entityId: string}>()` with
an optional context payload: `{ entityId?: string; listId?: string; viewId?:
string }`. `runCommand` builds `URLSearchParams` by switching on `cmd.context`
(mirroring the server's own switch) rather than unconditionally setting
`entity_id`. Existing mount sites keep working since `entityId` stays a prop.

2. **List surface** (`EntityList.vue`): fetch
`getCommands({pageType:'list', qualifier: props.listId})` alongside the existing
list-config load; render a `v-for` button group in the `<header
class="list-header">` at `:662-673` next to `+ New`; mount `<CommandModal
:list-id="props.listId" />`.

3. **Dashboard surface** (`DashboardView.vue`): fetch
`getCommands({pageType:'dashboard'})` in `loadData`; render buttons in the
dashboard header; mount a bare `<CommandModal />`.

4. **Backend validation fix** (`dataentryconfig/validate.go:1263`): in
`validateCommands`, reject an `available_on` that names a surface incompatible
with the declared `context` (e.g. `context: entity` + `lists:`, or `dashboard:
true` without `context: global`). This turns a confusing runtime 404 into a
startup config error. Chosen over relaxing `matchesPage` because the exec path
fundamentally needs one authoritative context per command.

**Alternatives considered:**
- *Make `matchesPage` consult `cmd.Context` as well as `available_on`* — rejected:
silently narrows existing configs, a behaviour change for anyone relying on
today's scope-overrides-context semantics. Validation surfaces it loudly
instead.
- *Reuse `useListActions` for list commands* — rejected: selection-scoped and
per-entity; would reintroduce the exact limitation being fixed.
- *Add a `/view/:id` route to complete all four contexts* — rejected as scope
creep; deferred to its own ticket.

**Files to modify:**
- `frontend/src/components/entity/CommandModal.vue` (generalise props + params)
- `frontend/src/components/lists/EntityList.vue` (fetch, render, mount)
- `frontend/src/views/DashboardView.vue` (fetch, render, mount)
- `internal/dataentryconfig/validate.go` (context/available_on coherence)
- `docs/data-entry.md` (document which contexts render where)
- Tests: `CommandModal.test.ts`, new `EntityList` + `DashboardView` specs,
`internal/dataentryconfig` validation tests

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- `list_id` / `view_id` from the SPA: server-side allowlist already — looked up in
`s.Cfg.Lists` / `s.Cfg.Views`, 404 on miss (`commands.go:314,325`). Client
cannot introduce an arbitrary id.
- Command id from the URL path: looked up in `s.Cfg.Commands`, 404 on miss.
- No new user-supplied data reaches `sh -c`; the script body stays config-authored.

**Security-Sensitive Operations:**
- This widens *where* configured commands can be triggered, not *what* can run or
*who* may run it. Same POST-only endpoint, same CSRF reasoning
(`commands.go:275-278`).
- Note: command execution has no ACL gate today — any session may invoke any
configured command. Pre-existing, unchanged by this ticket, and out of scope;
worth its own ticket (adjacent to FEAT-ESLP).
- No new error text exposes filesystem paths beyond what the entity surface
already does.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios (mapped to acceptance criteria):**
1. AC1 — component test: stub `getCommands`, mount `EntityList` with a scoped
command, assert button present; assert absent for a non-matching list id.
2. AC2 — mock `fetch`, click, assert URL is `/api/command/<id>?list_id=X`.
3. AC3 — same as AC2 with empty selection state; asserts the no-selection path.
4. AC4 — component test on `DashboardView`; assert POST carries no `entity_id`/`list_id`.
5. AC5 — existing `CommandModal.test.ts` must pass unchanged; add an assertion
that an `entity`-context command still sends `entity_id`.
6. AC6 — Go table test in `dataentryconfig`: each incoherent context/`available_on`
pairing yields a validation error; coherent ones pass.

**Edge Cases:**
- No commands configured → no button group, no empty container rendered
- `getCommands` rejects (network/500) → list still renders; failure must not blank
the page (mirror EntityDetail's abort-signal handling)
- Component unmounted mid-flight → abort signal, no state write after unmount
- `confirm:` set on a list command → confirm dialog before POST
- Command with `context: list` reached from a page whose type differs → params
follow `cmd.context`, which is the whole point of the modal change

**Negative Tests:**
- Unknown `list_id` → server 404, modal shows the error rather than hanging
- Non-POST to `/api/command/` → 405 (existing, keep covered)
- Incoherent config → startup validation error, not a runtime 404

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed
- [x] Effort estimated

**Risks:**
- *`CommandModal` prop change breaks the entity call site* — LOW: `entityId` stays
a supported prop; existing test pins the behaviour.
- *Validation fix rejects a config that works today* — MEDIUM, the only real risk.
A config relying on `available_on` overriding `context` would now fail at
startup. Mitigation: the only configs it breaks are ones that 404 at exec time
anyway, i.e. already broken but silently. Call it out in the PR description.
- *List header layout crowding with several commands* — LOW: same overflow
behaviour as the entity header; mirror its responsive treatment.

**Effort:** m

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] `docs/data-entry.md` — the `commands:` table (`:1691-1699`) must state which
contexts render where, and that `view` has no surface yet
- [x] N/A for metamodel.md / cli-reference.md — no metamodel or CLI change

## Design Review

- [ ] Run `/design-review` before starting implementation
- [ ] All critical/significant findings addressed in plan

**Design Review Findings:** <!-- pending -->
