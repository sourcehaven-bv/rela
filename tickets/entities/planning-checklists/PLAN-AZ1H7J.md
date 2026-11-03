---
id: PLAN-AZ1H7J
type: planning-checklist
title: 'Planning: Finish next actions: every page surface, context-aware sources, per-view and global placement config'
started: "2026-10-11"
completed: "2026-10-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

Three gaps make the next-action layer half-finished:

1. `NextActionCard` (banner/notice) is mounted only in `DashboardView.vue`.
On every other page a banner-band suggestion is invisible.
2. `context: <type>` sources are validated and documented but never resolved:
`nextActionCandidates` (`internal/dataentry/nextaction.go`) returns nil for
them, and the GET endpoint carries no notion of "the entity being viewed".
3. The operator has no say in where suggestions appear.

**Scope (in):**

- Page-level card mounted once in the app shell (`App.vue`), so every route
can show it. The status-bar chip stays where it is.
- The SPA resolves per page:
`GET /_next_action?surface=&view=&entity=&current=&world=`. No entity type from
the client: the server derives it from config.
- Surface kinds: `dashboard`, `list`, `kanban`, `calendar`, `gantt`, `entity`,
`document`, `page`, `form`, `search`, `other` (settings, analyze, history,
conflicts, apps, configure). Mapping is by route name with a fallback to
`other`.
- Global config: top-level `next_action_surfaces:` maps a surface kind to a
bool. Unset kinds are enabled, except `other`, which defaults to off.
- Per-view opt-out/in: `suggestions: <bool>` on `lists`, `kanbans`,
`calendars`, `gantts`, `views` (entity detail), `documents`, `pages` and
`dashboard`. Precedence: view bool, then surface bool, then the default.
- Per-source placement: `visible_on: [list, "list:open-tasks", entity, ...]`
on a source, mirroring `visible_worlds`. Empty means everywhere; an empty
written list is rejected.
- Stickiness: the client sends the key of the suggestion on screen as
`current=`. The engine keeps it, exempt from cooldown, while it is still
eligible here and no higher band fires. So one suggestion stays put while the
user navigates.
- Context-aware sources resolve whenever the request names an entity and the
surface implies its type (entity detail: route type; entity-anchored
documents/pages and edit forms: their configured type). The candidate is the
viewed entity, read by bare id in the source's `source_world`, verdicts primed,
field-redacted, then filtered by `condition:` through the gated matcher.
Impression cooldown does not apply to context sources; snooze, dismiss and mute
do.
- Re-resolve on the SSE change event for the context entity.
- Docs: `docs-project` data-entry guide (generates `docs/data-entry.md`).

**Scope (out):**

- New prominences, multi-slot rendering, analytics log (RES-09YLLL decision 12).
- Inferring relevance from a list's entity type; `visible_on` is explicit.
- The entity side panel (not a route), custom app bridge, per-tab and
per-space placement.
- `configedit` UI for the new keys beyond the allowlist entries.

**Acceptance Criteria:**

1. A banner-band suggestion appears on a list, kanban, entity detail and
document page, not only on the dashboard.
2. A `context: task` source fires on a task's detail page for that task,
honours its `condition:`, and does not fire on the dashboard or a list.
3. A context source does not fire for an entity the principal cannot read;
the response is identical to the one for a nonexistent id.
4. `next_action_surfaces: { list: false }` hides suggestions (card and chip)
on every list; a list with `suggestions: true` still shows them.
5. A source with `visible_on: [entity]` is eligible only on entity pages;
`"list:open-tasks"` only on that list.
6. Config load rejects an unknown surface kind, an unknown `visible_on`
surface or view, and an empty `visible_on`. An unknown `surface` on the request
is a 400; an unknown `view` answers empty with a server warning.
7. Navigating A -> B -> C with the same eligible sources keeps the same
suggestion on screen; the impression is reported once.
8. A stale response for a previous page never renders on the current one.
9. Without placement config, suggestions show on every surface except `other`.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: RES-09YLLL already covers the model; this ticket wires surfaces)
- [x] ~~Searched for existing libraries~~ (N/A: internal wiring, no library fits)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: placement config is rela-specific)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-09YLLL (via FEAT-79DTF9)

**Existing Solutions:**

- `DocumentConfig.IsStandalone()` discriminates entity-anchored vs standalone;
the same discrimination decides whether a request has an entity context.
- `findViewByEntityType` (`internal/dataentry/views_handler.go`) resolves the
detail view for a type; the entity surface uses it to find per-view config.
- `readAddressedOr404` + `a.visibleReader` is the gated single-entity read;
the context candidate reuses the visible reader (not the 404 writer).
- `nextActionSourceWorld` already scopes candidate reads to `source_world`.
- `permitsGatedUIElement` shows how presentation filters are kept separate
from boundaries; placement is presentation only, documented as such.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Config (`internal/dataentryconfig`):

- `NextActionSurface` closed enum + `Config.NextActionSurfaces map[NextActionSurface]bool`
(raises the `Config` plimsoll pin 26 -> 27, the documented format-mirror
exception).
- `Suggestions *bool` field (`yaml:"suggestions"`) on List, Kanban, Calendar,
Gantt, ViewConfig, DocumentConfig, Page, DashboardConfig.
- `NextActionSource.VisibleOn []string` with `VisibleOnSurface(surface, view)`.
- `Config.NextActionsEnabled(surface, viewID) bool` owns the precedence.
- Validation in `nextaction_validate.go`.

Engine (`internal/nextaction`):

- `WithSourceFilter(func(id string, src) bool)`; `sourceIDsForBand` skips
filtered sources before mute lookups or reads.
- `WithCurrent(userstate.Key)`: that key bypasses cooldown when it is
produced again; within its band it is picked over the random choice.
- Context sources skip the impression cooldown check.

Data entry (`internal/dataentry`):

- GET parses `surface`, `view`, `entity`, `current`. Disabled -> empty answer
before building the engine. Filter = visible_worlds + visible_on + context type
matches the surface-derived entity type.
- Context candidate: closure captures the bare id; inside the rebound
source-world ctx it reads through `a.visibleReader`, primes verdicts, redacts.
Miss or hidden -> nil; gate infrastructure errors propagate.
- Entity surface per-view lookup: `findViewByEntityType(cfg.Views, view)`.

Frontend:

- `nextActionPlacement.ts`: route -> `{surface, view, entity}` table and a
`enabledFor(config, placement)` mirror used only to skip the request.
- `useNextAction`: latch on world + placement; AbortController plus a
sequence guard; send `current`; hide while the tuple changes; re-resolve on SSE
change of the context entity.
- Impression watchers key on the suggestion key, not a boolean.
- `NextActionCard` moves from `DashboardView.vue` to `App.vue` above `RouterView`.

**Alternatives considered:**

- Mount the card in each view: rejected, eight call sites that drift.
- Per-view `sources:` allowlists: rejected in design review; placing a source
would mean editing every view. `visible_on` on the source follows
`visible_worlds` and `CommandScope`.
- Client-side stickiness only: rejected, the server would still put the shown
suggestion in cooldown and return a different one.
- Infer eligible sources from a list's entity type: rejected, guesses quietly.

**Files to modify:**

- `internal/dataentryconfig/{nextaction.go,nextaction_validate.go,config.go,page.go}` + tests
- `internal/nextaction/nextaction.go` + tests
- `internal/dataentry/{nextaction.go,nextaction_handler.go}` + tests
- `internal/configedit/allowlist.go` (new keys)
- `frontend/src/composables/useNextAction.ts`, new `nextActionPlacement.ts`,
`frontend/src/App.vue`, `DashboardView.vue`, `NextActionCard.vue`,
`SidebarFooter.vue`, `api/entities.ts`, `types/config.ts` + tests
- `docs-project/entities/guides/GUIDE-data-entry.md` (regenerate `docs/`)
- e2e: `e2e/tests/next-action-surfaces.spec.ts`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `surface`: closed enum, 400 on anything else.
- `view`: must name a configured view of that surface kind, 400 otherwise.
Config names are not secret (root CLAUDE.md), so naming it is fine.
- `entity`: parsed by `ParseRef`, face dropped; never echoed. A miss, wrong
type or hidden entity yields no candidate, so the endpoint is no existence
oracle. Gate infrastructure errors still propagate (RR-4TFZNL rule).
- `current`: only a hint; it can keep a suggestion that is independently
eligible, never produce one. Unknown or ineligible keys are ignored.
- `visible_on` and surface keys in config: validated at load.

**Security-Sensitive Operations:**

- The context read goes through `visibleReader` (row gate) and
`redactedForSuggestion` (field gate) before interpolation or condition, same
order as query candidates. A condition on a hidden field sees Nil.
- Placement is presentation only: it narrows what is shown, never what may be
read. Documented as such.
- No caching (unchanged; per-principal results).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- AC1: e2e spec loads list, kanban, entity and document routes with a banner
source and asserts `.rela-na` is visible on each.
- AC2: handler test with a `context: task` source + condition: fires for a
matching task with `entity=`, not for a non-matching one, not without `entity=`.
- AC3: handler test with an ACL hiding the task: response equals the response
for a random nonexistent id (byte-compare).
- AC4/AC5: config unit tests for `NextActionsEnabled` and `VisibleOnSurface`;
handler tests for disabled and `visible_on`; frontend test for chip + card
hidden.
- AC6: validation tests; handler 400 tests.
- AC7: engine test for `WithCurrent` under cooldown; composable test A->B->C.
- AC8: composable test with deferred promises resolving out of order.
- AC9: handler test with no placement config on every surface kind.
- Budget: `storetest.Counting` test that a disabled surface makes zero reads.

**Edge Cases:**

- `entity` present with `surface=list` (ignored for placement; context sources
still need a matching type, so only entity surfaces get them).
- Context entity whose type differs from `context:` -> no candidate.
- Faced address `ID@published` with an editorial source: editorial face evaluated.
- `visible_on: []` written explicitly -> rejected at load.
- Context source with `count`/`query` -> already rejected by validation.
- Same suggestion on two pages -> impression counted once (shownKey).

**Negative Tests:**

- Unknown surface (400), unknown view (empty + warn), unknown surface or view
in `visible_on`, `suggestions: 3`.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Behaviour change: banners now appear beyond the dashboard. Mitigation:
documented, and one global key turns any surface off.
- Extra request per navigation. Mitigation: short-circuit engine, latch on
the tuple, disabled placement answers before any query.
- Route-to-surface table drifting from the router. Mitigation: unit test
iterates every named route, including `/s/:space` aliases.
- Full-height views pushed down by the banner. Mitigation: e2e at a fixed viewport.

Effort: l

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] docs/data-entry.md (via docs-project guide): "Where suggestions appear"
section, `next_action_surfaces`, per-view `next_actions`, context sources.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 16 review-responses linked to TKT-6EEXXE (1
critical, 7 significant), all addressed in this plan.
