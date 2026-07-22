---
id: PLAN-CNDJ78
type: planning-checklist
title: 'Planning: Render list/view/global commands in the data-entry SPA (backend serves all four contexts, frontend renders only entity)'
status: done
---

<!-- @managed: claude-workflow v1 -->

> **DESIGN REVIEW COMPLETE — 3 critical, 2 significant, 1 minor.**
> The Security Considerations section below was **factually wrong** in its
> first draft and has been rewritten.
>
> **BLOCKING DECISIONS RESOLVED (2026-07-20).** Decision A is answered by
> **TKT-MJ02AO** (per-command ACL guard), which this ticket now
> `depends-on`. Decision B is folded into that ticket. TKT-72SCPR stays in
> `planning` until TKT-MJ02AO lands — see "Sequencing" below.

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: generalize `CommandModal.vue` off its `entityId` prop (as a **closed
union**, not an open map — RR-KNEF4K); extract a `useCommands()` composable +
`CommandButtons.vue`; mount on dashboard and list surfaces; fix the
command-stream cancellation leak; strengthen two thin Go SSE tests; e2e
click-through for dashboard + list.

OUT: Lua command backend (TKT-XTNED); **all ACL/permission work → TKT-MJ02AO**;
changes to command config schema or SSE protocol; API-level Playwright specs
(prohibited by `e2e/tests/AGENTS.md:45`, already covered in Go).

**Re-scoped by review:** the `available_on.views` fix is not a simple "pass
`qualifier`" (RR-N643V4). See Approach.

## Sequencing (post-review)

**TKT-MJ02AO must land first.** It adds a `permission:` key to `CommandConfig`,
403s unauthorized exec, denies all command exec under `ReadOnlyACL`, and filters
`resolveCommands` by held permission.

Two things follow for this ticket once it lands:

1. **Button visibility comes for free.** `GET /_commands` returns only
commands the principal may execute, so `CommandButtons.vue` renders a
permission-filtered list with no client-side ACL logic. Do **not** add
client-side permission checks — the server filter is the single source of truth,
and the 403 is the boundary.
2. **The read-only e2e assertion becomes writable.** TKT-MJ02AO corrects
`e2e/tests/read-only-mode.spec.ts:8`; this ticket can then assert that no
command buttons render under `--read-only`, which is currently false.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: backend contract already exists and dictates the frontend shape)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — see above.

**Existing Solutions:**

No library applies; internal wiring against an existing in-tree API.

Prior art reused rather than reinvented:

- `composables/useScopeNavigation.ts` — the convention. Named export, reactive
params as **getter functions** (`useScopeNavigation.ts:21`, called at
`EntityDetail.vue:64` as `useScopeNavigation(() => props.entityId)`), returns
refs + functions, loader invoked explicitly by the consumer.
- `composables/useEvents.ts` — existing SSE composable with connection lifecycle.
- `EntityDetail.vue:312-331` — BUG-6C3V abort pattern (double guard: explicit
`signal.aborted` **and** `isCancelledFetch(err)`). Preserve verbatim.
- `EntityList.vue:653-664` — existing `.list-header` flex header.

**Findings that changed the plan:**

1. **`ListView.vue` is an 11-line pass-through** (`<EntityList :list-id="id" />`).
The list command row belongs in `components/lists/EntityList.vue`.

2. **The `entityId` coupling is one line** — `CommandModal.vue:42-45` is the
only use of `props.entityId`. The real cost driver is avoiding triplication of
~85 lines across three surfaces.

3. **Command-stream cancellation leak** (pre-existing). `EntityDetail.vue:648`
aborts only the *list* fetch; `CommandModal.vue` has no `AbortController` and no
`onBeforeUnmount`. **In scope** — see Risk 4.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **`composables/useCommands.ts`** — `useCommands(ctx: () => GetCommandsParams)`.
Owns the abortable list fetch (lifting `EntityDetail.vue:312-331` verbatim),
exposes `commands` ref + `loadCommands()`.

2. **`components/commands/CommandButtons.vue`** — desktop row, mobile overflow
menu, outside-click watcher (`EntityDetail.vue:304-310`), `CommandModal` ref,
`runCommand` delegation. One modal instance **per surface** (design-review
confirmed this seam; a shared singleton would race).

3. **`CommandModal.vue`** — replace `entityId: string` with a **closed
discriminated union** (RR-KNEF4K), NOT `Record<string,string>`:
   ```ts
   type CommandParams =
     | { entity_id: string }
     | { entity_id: string; view_id: string }
     | { list_id: string }
     | Record<string, never>   // global
   ```
Keeps `exec_id` structurally unreachable from any call site.

4. **Cancellation** — add an `AbortController` around the exec stream and abort
it in `onBeforeUnmount`; allow Close while running.

**`available_on.views` (RR-N643V4) — re-scoped.** Passing `qualifier` with
`pageType: 'entity'` fixes nothing: `matchesPage` `case "entity"`
(`commands.go:94-97`) reads only `scope.EntityTypes`. The real fix requires
sending `pageType: 'view'` when a view is active, which also changes which
unscoped and `context: view` commands resolve. **Decide during
implementation-planning whether to take that behavioral delta or drop the AC.**
Do not ship the no-op version.

Call sites: `EntityDetail.vue` (migrate existing), `EntityList.vue` (`pageType:
'list'`, `qualifier` = `props.listId`), `DashboardView.vue` (`pageType:
'dashboard'`, no qualifier).

**Alternatives considered:**

- *Copy the block into each surface.* Rejected: triplicates BUG-6C3V.
- *Composable only, no component.* Rejected: leaves ~35 template lines per surface.
- *`params: Record<string,string>`.* **Rejected by review (RR-KNEF4K)** — an
open map into a `sh -c` endpoint's query string.
- *Client-side permission filtering.* **Rejected** — TKT-MJ02AO filters
server-side; duplicating it client-side invites drift and implies the client is
a boundary.
- *Move exec into `api/commands.ts`.* Deferred; follow-up.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

> **REWRITTEN AFTER DESIGN REVIEW.** The original claimed "the server already
> authorizes these." That is false. Three critical findings came from that one
> wrong assumption. **TKT-MJ02AO makes it true**; until it lands, the
> statements below describe the live system.

**Security posture of command execution (pre-TKT-MJ02AO):**

1. **No authorization on command execution.** (RR-65KG68) `handleCommandExec`
(`commands.go:281`) does four things before `sh -c`: method check, command-ID
lookup, stdin build, exec. Grep for
`acl|ACL|translateVerb|WriteRequest|Principal` across `commands.go` and
`command_handler.go` → **zero matches**. Only gates are
`requireSameOrigin`/`requireLocalHost` (`router.go:105-108`) — CSRF and
network-location, not authorization.

2. **`--read-only` does NOT block command execution.** (RR-CWWJGW)
`acl.ReadOnlyACL` (`internal/acl/readonly.go:18`) implements exactly one method,
`AuthorizeWrite(WriteRequest)`. Command exec constructs no `WriteRequest`. There
is also **no frontend read-only state** — the SPA hides Edit/Delete/+New because
the *server omits affordances*, not because the client knows the mode.

3. **`available_on` is display scoping, NOT an authorization boundary.**
(RR-L6UXCF) `matchesPage` runs only on the GET `/_commands` render path; exec
never validates the supplied IDs against `cmd.AvailableOn`.

4. **`validateCommands` (`validate.go:1255`) is config-load validation only.**
It checks referenced IDs exist in config; it establishes nothing at request
time. Citing it as the input-validation story was the original error.

**What this ticket changes, given the dependency:**

With TKT-MJ02AO landed first, this ticket renders buttons against a surface that
is genuinely gated: unauthorized principals get a filtered command list and a
403 if they forge a request. The dashboard mount is then safe to ship.
**Shipping this ticket before TKT-MJ02AO would put an ungated shell-exec button
on the first page every user lands on** — hence the `depends-on`.

**Preserved boundaries (do not regress):**

- `cmd.label` renders via text interpolation (`{{ cmd.label }}`), never
`v-html`. XSS boundary for admin-authored config.
- POST-only on exec (`commands.go:282`) — the `<img src>` RCE fix documented
at `commands.go:274-279`.
- CSRF Origin allowlist, covered in Go by `middleware_security_test.go` /
`router_security_test.go`. Per AGENTS.md do **not** add an e2e equivalent.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Layering rule (`e2e/tests/AGENTS.md:45`):** HTTP-shape assertions in Go;
click-through journeys only in Playwright. `request.fetch(...)` /
`api.rawRequest(...)` are eslint errors in `tests/**/*.spec.ts`.

| AC | Layer | Scenario |
|---|---|---|
| global renders + executes | vitest + e2e | dashboard button mounts; e2e clicks, modal opens |
| list renders, `available_on.lists` scoping | vitest + e2e | scoped command present for matching list, **absent** for non-matching |
| view scoping | vitest | **REWRITTEN — see below** |
| `CommandModal` without entity | vitest | mount with `{}` params; assert URL has no `entity_id` |
| entity behavior unchanged | vitest | existing `CommandModal.test.ts` green with only the prop-shape edit |
| cancellation | vitest | unmount mid-stream aborts the fetch; Close enabled while running |
| Go SSE assertions | Go | strengthen `TestHandleCommandExecListContext:713` / `ViewContext:733` |
| global env contract | Go | assert `RELA_ENTITY_ID` **absent** (not empty) for global input (RR-I3VLJV) |
| read-only | e2e | no command buttons render under `--read-only` (**requires TKT-MJ02AO**) |
| e2e click-through | e2e | dashboard + list |

**REWRITTEN per RR-N643V4 — the original view-scoping test was unfalsifiable.**
It asserted `getCommands` was *called with* a qualifier, i.e. it asserted the
mock, and would pass green against a broken implementation. Replace with: seed a
command with `available_on.views: [X]`, render on view X → assert the button
**appears**; render on view Y → assert it **does not**. Assert the resolved
command list, never the call arguments.

Frontend convention: **`.test.ts` co-located**. Mock via `globalThis.fetch` spy
with the teardown trio from `CommandModal.test.ts:52-68` (restore fetch, clear
`document.body.innerHTML`, reset modal/confirm singletons). Modal DOM queried
via `document.querySelector` (`attachTo: document.body`).

**Edge Cases:**

- Empty command list → no button row, no overflow button
- **Permission-filtered to empty** (post-MJ02AO) → same as empty; assert no stray container renders
- Dashboard has no qualifier — assert the param is **omitted**, not sent empty
- Rapid navigation between lists → BUG-6C3V abort path
- `DashboardView` has no abort controller today (`onMounted` only) — the composable introduces the pattern fresh there
- Long `cmd.label` on mobile → overflow menu
- Command with `confirm:` on a non-entity surface → confirm modal still gates
- Unknown/stale qualifier → empty set, fail-closed
- `natsort` (`commands.go:58`) means button order shifts on rename — pin e2e assertions or they flake

**Negative Tests:**

- Non-matching `available_on.lists` → button absent (paired positive/negative
fixtures, per `feature_summary`/`feature_readonly` at `fixtures.ts:1030-1051`)
- Command list fetch 500s → `commands = []`, no throw, page still renders
- SSE `error` event → `success=false`, error text shown
- Aborted fetch → silently ignored, no console error

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

1. **Regressing shipped entity-detail behavior.** *Mitigation:* migrate it
last; keep `CommandModal.test.ts` green throughout as the canary.

2. **Shared `e2e/tests/fixtures.ts`.** `fixtures.ts:1039-1046` warns that
adding config shifts counts asserted in `settings.spec.ts` and
`entity-detail.spec.ts`. `createTestProject()` takes no parameters — no per-test
override. *Mitigation:* run the full e2e suite before and after; budget for
touching those assertions. **Coordinate with TKT-MJ02AO**, which will also touch
this fixture to add `permission:` cases.

3. **Type union mismatch** — `GetCommandsParams.pageType` uses `'dashboard'`,
`Command.context` uses `'global'` (`api/commands.ts:5` vs
`types/config.ts:387`). Writing `pageType: 'global'` yields an empty list and no
error. *Mitigation:* a `const` map the compiler enforces, **not** a comment.

4. **Cancellation leak — IN SCOPE.** With three surfaces, rapid navigation
spawns unbounded `sh -c` processes: `commands.go:360` binds to `r.Context()`,
but nothing client-side aborts the stream, so the request stays open and the
process lives. `runningCommands` grows; `Close` is `:disabled="running"`
(`CommandModal.vue:169`) so the user cannot dismiss them.

5. **`DashboardView` header needs a flex wrapper** — `h1` + `p` are stacked
blocks with no actions container, unlike `.list-header`.

6. **Dependency slip.** If TKT-MJ02AO is delayed, the temptation will be to
ship the frontend anyway. *Mitigation:* the `depends-on` relation plus the
read-only AC, which cannot pass without it.

**Effort:** `m` — confirmed. The ACL work moved to TKT-MJ02AO (`l`), so this
stays frontend-plus-tests as originally sized.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/data-entry.md` — **required.** Correct wording about which contexts
render, and note that `context: global` scripts receive **no**
`RELA_ENTITY_ID`/`RELA_ENTITY_TYPE`, with a `set -u` recommendation (RR-I3VLJV).
The `permission:` key and the `available_on`-is-not-authz note are documented by
TKT-MJ02AO.
- [ ] ~~docs/metamodel.md~~ (N/A: no metamodel change)
- [ ] ~~docs/cli-reference.md~~ (N/A: no CLI change)
- [ ] ~~CLAUDE.md~~ (N/A: follows existing composable pattern)
- [ ] ~~README.md~~ (N/A: not project-level)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

| ID | Severity | Title | Resolution |
|---|---|---|---|
| RR-65KG68 | critical | No authorization layer on command exec | deferred → TKT-MJ02AO |
| RR-CWWJGW | critical | `--read-only` does not block command exec | deferred → TKT-MJ02AO |
| RR-N643V4 | critical | `available_on.views` fix doesn't work; test unfalsifiable | addressed in plan |
| RR-L6UXCF | significant | `available_on` not enforced at exec time | deferred → TKT-MJ02AO |
| RR-KNEF4K | significant | Open `params` map into shell-exec query string | addressed in plan |
| RR-I3VLJV | minor | `RELA_ENTITY_ID` absent for global commands | addressed in plan |

**Resolved open questions:**

1. *Cancellation leak* → **fix here.** A direct consequence of going from one
surface to three. Risk 4.
2. *Move exec into `api/commands.ts`* → **defer.** But verify the raw `fetch`
at `CommandModal.vue:48` isn't missing an interceptor the axios client applies.
3. *Is `CommandButtons.vue` the right seam* → **yes.** One modal per surface.

**Blocking decisions — RESOLVED:**

- **A. Read-only mode** → **option 1, gate exec on ACL**, scoped to
**TKT-MJ02AO** as a prep PR. Per-command configurable `permission:`, plus a
blanket deny under `ReadOnlyACL`, plus permission-filtered resolution so buttons
hide.
- **B. Exec-time scope enforcement (RR-L6UXCF)** → **folded into TKT-MJ02AO**,
same file and review.
