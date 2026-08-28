---
id: IMPL-XJCM01
type: implementation-checklist
title: 'Implementation: Permission-based dashboard card filtering (UX: hide cards a user cannot use)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

**What was built**

| Layer | Change |
|---|---|
| Config | `Permission` on `DashboardCard` (`dataentryconfig/config.go`), doc block mirroring `NavigationEntry.Permission` |
| Policy | Extracted `permitsGatedUIElement(ctx, acl, permission)` from `permitsNavEntry`, which is now a one-line wrapper. ONE switch, so the read-only arm cannot drift |
| Endpoint | `GET /api/v1/_dashboard` (`dashboard_handler.go`), registered beside `_sidebar`. Always 200; `cards` initialised via `make(...)` so the wire carries `[]`, never `null` |
| Wire | `v1.DashboardResponse` |
| Guard | `TestNavFilterStaysPresentational` now greps TWO needles with separate allow-lists |
| SPA | `api/dashboard.ts`, store loads `/_dashboard` in the existing `Promise.all`, `DashboardView` empty state + loading gate |
| Docs | `docs/data-entry.md`, `docs/acl-security.md`, `internal/dataentry/CLAUDE.md` (incl. the stale `TestNavPermission_ReadOnlyHides` fix) |

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Go tests reuse the existing `gatedNavPolicy()` and add
`installGatedDashboardConfig` / `installDashboardConfig` builders. Frontend
tests use a `card()` factory.

**Mutation-verified** (a passing test proves nothing until seen to fail):

| Mutation | Tests that failed |
|---|---|
| Remove the `ReadOnlyACL` arm | `_ReadOnlyShowsEverything` (both forms), `_ReadOnlyArmIsExplicit` |
| Invert the filter predicate | `_HolderSeesGatedCard`, `_NonHolderFiltered` (incl. order), `_FilterIsPresentationOnly` (both directions) |
| Narrow the grep allow-list | `TestNavFilterStaysPresentational`, naming the exact file |
| Read `configData.dashboard` instead of `/_dashboard` | `schema.test.ts` "loads schema and config from API" |
| Drop the store-load await | `DashboardView` "shows the loading state, not the empty state" |
| Remove the empty state | `DashboardView` "shows the empty state when there are no cards" |

The `/_config` assertion in `schema.test.ts` was made **discriminating**: the
two sources now return different cards, so it can actually tell which one the
store read. Previously both returned `{cards: []}` and it would have passed
either way.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Live `rela-server` against a scratch project with a real `acl.yaml`
(`alice`=admin holding `admin:read`, `bob`=viewer, `carol`=auditor holding the
permission but granted NO reads), 3 cards: `Open Tickets`, `Audit Log`
(`permission: admin:read`), `Recent`.

| AC | Evidence | Result |
|---|---|---|
| 1 | ungated cards for alice/bob/no-ACL | PASS — always present |
| 2 | alice `[Open, Audit Log, Recent]`; bob `[Open, Recent]` | PASS |
| 2 | bob's survivors keep configured order | PASS |
| 3 | `--read-only`, bob (non-holder) | PASS — `Audit Log` still shown |
| 3 | `acl.yaml` removed entirely, bob | PASS — `Audit Log` still shown |
| 5 | bob: card HIDDEN yet `/tickets` returns `total:1` | PASS — hiding gates nothing |
| 5 | carol: card SHOWN yet `/tickets` returns `total:0` | PASS — showing grants nothing |
| 6 | `/_config` byte-identical alice vs bob, still carries `"Audit Log"` | PASS |
| 7 | no `dashboard:` → `{"cards":[]}`; all-filtered → `{"title":"Overview","cards":[]}` | PASS — `[]` not `null` |

**UI verified in Chrome** against two servers pinned to different identities:
- alice: three cards including `Audit Log`.
- bob: two cards, `Audit Log` absent, grid reflows cleanly with no gap.
- all-filtered: renders "No dashboard cards to show." — not a bare heading.

Note during setup: an `acl.yaml` placed in `.rela/` is silently ignored (it
loads from the project root), which presents as NopACL — worth knowing, since
the symptom is "filtering does nothing".

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

DRY: the whole point of the design is that the ACL switch exists **once**.
`permitsNavEntry` was reduced to a wrapper rather than duplicated, which is what
makes the RR-XYO03L read-only bug structurally unrepeatable across the two
surfaces. Deliberately NOT extracted: the per-surface filter loops, which are
three lines each and differ in what they iterate.

Security: this adds no enforcement and is documented throughout as a UX filter.
`_search`/list endpoints are untouched; AC5 pins presentation-only in both
directions. The new endpoint serves strictly less than the `/_config` every
principal already receives.

**Full suite green:** `go test -race ./internal/...` (all pass), `just lint` (0
issues), `just arch-lint` (OK), `just plimsoll` (OK), `just coverage-check`
(76.9%, thresholds satisfied), frontend `npm run test:run` (1537 passed), `npm
run lint` (0 errors), `npm run build` (typecheck + bundle OK).
