---
id: PLAN-X3SW5Y
type: planning-checklist
title: 'Planning: Permission-based dashboard card filtering (UX: hide cards a user cannot use)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

The user asked to "filter dashboard cards similarly to nav items (so for UX
purposes) via a permission". TKT-TXDK8U already did exactly this for sidebar nav
entries; this ticket extends the same affordance to `dashboard.cards`.

**The rationale is UX, not confidentiality**, and that must not drift. A card's
data already flows through the ACL-scoped `_search` path (`helpers.go:411`,
`SearchScope`), so a principal who may read nothing already sees `count: 0` / an
empty table. What they *also* see today is the card title, the empty shell, and
a `↗` link exposing the raw query. Hiding the card removes a useless tile; it
conceals nothing, because `data-entry.yaml` is an operator-authored repo file
(root CLAUDE.md, "The configuration is not a secret; the data is").

**Scope:**

IN:
- `Permission string` on `DashboardCard` (`dataentryconfig/config.go:625-634`),
with a doc block mirroring `NavigationEntry.Permission` (`config.go:526-548`).
- A new principal-scoped `GET /api/v1/_dashboard` endpoint serving the resolved,
filtered card list.
- Extracting the ACL switch out of `permitsNavEntry` so nav and dashboard share
ONE copy (see Approach).
- SPA: `DashboardView.vue` reads the new endpoint instead of
`schemaStore.dashboard`; empty-state when every card is filtered.
- Docs: `docs/data-entry.md` card table, `docs/acl-security.md`,
`internal/dataentry/CLAUDE.md`.

OUT:
- Validating `permission:` values against `acl.yaml` — deferred for ALL
consumers in RR-2KZEXF; this adds a fourth to the same known gap. Documented,
not fixed here.
- Hiding cards on a zero ACL-scoped result count (declined in TKT-TXDK8U: makes
the UI flicker as data changes).
- Hiding the "Dashboard" sidebar entry when all its cards are filtered (user
chose card-level only; it would couple the sidebar to dashboard evaluation).
- The hardcoded validation-summary card (`DashboardView.vue:206-234`) — it is
not a configured card and has no `permission:` to carry.
- Any new `permission:` surface on `list:`/`kanban:`/`action:`.
- **A multi-principal E2E fixture** (RR-PZKLVV). None exists today; building one
is its own ticket.

**Acceptance Criteria:**

1. A card with no `permission:` renders for everyone; the check short-circuits
before any ACL work.
2. Under `*acl.Declarative`, a card with `permission: x` renders only for a
principal holding `x`.
3. Under `NopACL` **and** `ReadOnlyACL` (value *and* pointer forms), gated cards
**render**.
4. A nil ACL, or an unknown `acl.ACL` implementation, **hides** the card.
5. The filter is presentation-only: `_search` behind a hidden card's query
returns exactly what it returned before.
6. `/api/v1/_config` stays byte-identical across principals, `dashboard:`
included.
7. A dashboard with no surviving cards returns **200 with `cards: []`** and the
SPA renders an empty state — never `null`, never 404 (RR-CLZB5I).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: 'm'-sized change with a landed, reviewed in-tree analogue in TKT-TXDK8U; no approach survey needed)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — see above. No library question: this is project-internal
ACL policy.

**Existing Solutions:**

- `permitsNavEntry` — `internal/dataentry/views_handler.go:403-429`, plus its
godoc (351-402). This is the reference implementation: the ordered policy, the
read-only reasoning, the closed switch. Reuse, do not copy.
- `handleV1Sidebar` — `views_handler.go:192-257`. The endpoint shape to mirror:
ACL resolved ONCE per request (line 210), filter per item (220/236), drop
emptied containers (226-232), and **`make([]T, 0)` at line 209 so the wire
carries `[]` not `null`**.
- `NavigationEntry.Permission` doc block — `dataentryconfig/config.go:526-548`.
The wording to mirror on `DashboardCard.Permission`.
- Group-level rejection — `dataentryconfig/validate.go:236-245`: `permission:`
on a *group* is a config error. Precedent for where a permission may sit.
- `resolveCommands` (`commands.go`) / `_actions` — the established "return only
what this principal can use, endpoint re-checks" shape.
- Review history that shaped the policy: **RR-XYO03L** (critical — copying
`authorizeCommand`'s read-only deny arm hid entries from *everyone*) and
**RR-CWWJGW** (`nopReadGate.HoldsPermission` returns true, so a predicate
written against the read gate alone fails OPEN under `--read-only`).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

*1. Config.* Add `Permission string \`yaml:"permission,omitempty"
json:"permission,omitempty"\``to`DashboardCard`, doc block mirroring
`NavigationEntry.Permission`. No new validation rule (an unknown permission name
is the RR-2KZEXF gap, out of scope); `validateDashboard` is untouched.

*2. Share the policy, don't duplicate it — but keep the guard's teeth*
(RR-QAEM5Z). Extract the switch from `permitsNavEntry` into:

```go
func permitsGatedUIElement(ctx context.Context, aclImpl acl.ACL, permission string) bool
```

Naming is deliberate: a general name like `permitsByPermission` reads as a
general-purpose authorization helper and is one import away from a write path —
the exact drift TKT-M1AX6P was reverted for. `permitsGatedUIElement` reads
*wrong* anywhere but a presentation path. The full "presentation only" godoc
moves onto this function (not the wrapper), and states that a caller wanting
authorization must use `authorizeCommand`. `permitsNavEntry` becomes a one-line
wrapper.

`lint_test.go:68-110` gets **two needles, not one widened rule**:
`permitsNavEntry(` stays pinned to `views_handler.go` alone;
`permitsGatedUIElement(` is allowed in `views_handler.go` + the new dashboard
handler. Correcting the earlier draft: widening is *not* "the intended way to
use" that test — its doc says needing the predicate elsewhere is "the moment to
stop and ask whether you want an authorization check instead". This is a
deliberate, argued exception: one shared switch means the RR-XYO03L read-only
arm cannot drift between two surfaces, which is worth more than a one-file
allowlist.

*3. New endpoint.* `GET /api/v1/_dashboard` on `viewsHandler` (it already holds
the `aclImpl` closure and `currentACL()`), registered beside `_sidebar` at
`api_v1.go:99`. Returns `v1.DashboardResponse{Title, Description, Cards}`.
Resolve the ACL once, filter, return. **Always 200** — `cards :=
make([]v1.DashboardCard, 0)` so a missing `dashboard:`, `cards: []`, and
all-filtered all produce the same `[]` (RR-CLZB5I). No 404: it would need an SPA
branch for a non-error, and it would collide with the real meaning of 404
(broken route). `/_config` keeps serving `s.Cfg.Dashboard` verbatim — untouched.

*4. SPA.* New `api/dashboard.ts` client; `DashboardView.vue` gets its card list
from `/_dashboard` instead of `schemaStore.dashboard`. Server-side filtering
keeps indices dense, so the index-keyed `cardData` map needs no remapping.

Per RR-TIO1XP, two things are decided rather than left to accident:
- **Caching:** load `/_dashboard` once into the schema store alongside the
existing mount-time `/_config` load, rather than per visit — this preserves
today's free repeat visits to `/dashboard` and matches the established store
pattern.
- **Loading state:** `loading` must cover the dashboard-config fetch, not just
the card queries. The new empty state and the not-yet-loaded state are visually
identical, so getting this wrong makes every load flash "no cards".

The card queries now depend on a round-trip that previously wasn't on the
critical path (`schemaStore.dashboard` was already resident at mount). Accepted:
one RTT against N parallel searches, and the store-level cache confines it to
first load.

**Alternatives considered:**

- *Per-card `permitted` boolean on `/_config`* — rejected. It still makes
`/_config` principal-dependent, failing `TestNavPermission_ConfigUnfiltered`
(`nav_permission_test.go:346-368`, byte-identical bodies), and it is the
concealment-shaped direction reverted once in TKT-M1AX6P.
- *Filter `Cards` inside `handleV1Config`* — rejected, same test failure, and it
directly contradicts `internal/dataentry/CLAUDE.md` ("Do NOT filter
`/_config`"). Would require deliberately amending a documented rule to save a
small endpoint.
- *Client-side filtering from a permission list* — rejected outright:
`internal/dataentry/CLAUDE.md` forbids a `useACL()` composable or any
client-side ACL evaluator (TKT-AWM6L wont-fix). The SPA reads booleans the
server computed.
- *Grey-out placeholder instead of hiding* — considered and rejected by the
user; it shows users precisely what they cannot use, inverting the UX goal.

**Files to modify:**

- `internal/dataentryconfig/config.go` — `Permission` on `DashboardCard`.
- `internal/dataentry/views_handler.go` — extract `permitsGatedUIElement`;
`permitsNavEntry` becomes a wrapper.
- `internal/dataentry/dashboard_handler.go` (new) — `handleV1Dashboard`.
- `internal/dataentry/api_v1.go` — route registration beside line 99.
- `internal/apiwire/v1/responses.go` — `DashboardResponse` / `DashboardCard`.
- `internal/dataentry/lint_test.go` — second needle + allowlist.
- `internal/dataentry/dashboard_permission_test.go` (new) — the AC tests.
- `frontend/src/api/dashboard.ts` (new), `frontend/src/stores/schema.ts`,
`frontend/src/types/config.ts`, `frontend/src/views/DashboardView.vue`.
- `frontend/src/views/DashboardView.test.ts` (new — no such file exists today).
- `docs/data-entry.md`, `docs/acl-security.md`,
`internal/dataentry/CLAUDE.md`.

**Opportunistic fix:** `internal/dataentry/CLAUDE.md` names the read-only canary
`TestNavPermission_ReadOnlyHides`; the real tests are
`TestNavPermission_ReadOnlyShowsEverything` and
`TestNavPermission_ReadOnlyArmIsExplicit`. The stale name is actively misleading
(it says "Hides" where behaviour is "Shows"). Correct it while editing that
section.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `card.permission` — operator-authored `data-entry.yaml`, trusted-ish config.
Any non-empty string is accepted; an unknown name yields a card nobody sees (the
RR-2KZEXF failure mode). Not validated here; documented as a gotcha with a
face to the roles' `permissions:` list, matching what TKT-TXDK8U did.
- Principal identity — from the existing request middleware, unchanged.
- No new user-supplied input. `card.query` is not newly trusted: it is
neither parsed nor executed by the new endpoint, which returns config only.

**Security-Sensitive Operations:**

- **This feature is not a security control and must not be described as one.**
The endpoints behind every card enforce exactly what they enforce today. A
hidden card's query typed into `/search` returns the same ACL-scoped rows it
always did. AC5 pins this in both directions (permission-denied but
data-permitted, and permission-held but data-denied), copying
`TestNavPermission_FilterIsPresentationOnly` — whose row-count assertion is the
load-bearing half, since asserting only HTTP 200 passes vacuously.
- **Fail-closed default.** nil ACL and unknown implementations hide. The
failure mode of forgetting an arm is a missing card, never an unintended one.
- **The read-only arm is the known trap.** `readGateFromContext` returns
`nopReadGate` under both NopACL and ReadOnlyACL, and its `HoldsPermission`
returns `true` unconditionally — so falling through to the gate fails OPEN while
appearing to check (RR-CWWJGW). Keeping the explicit arm ahead of the gate is
load-bearing; reusing the shared predicate inherits it for free.
- **Error handling:** the endpoint returns a filtered card list, always 200 for
a well-formed GET. No 403-vs-404 distinction is needed — this endpoint is not an
entity read, and config names are not secrets.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

Per RR-PZKLVV, each layer is claimed only for what it can actually prove.

**Layer 1 — Go handler tests** (`dashboard_permission_test.go`, mirroring
`nav_permission_test.go`; helper `installGatedDashboardConfig`, reusing
`gatedNavPolicy` — alice holds `admin:read`, bob does not). This is where the
policy is proved.

| AC | Test | Scenario |
|---|---|---|
| 1 | `TestDashboardPermission_UngatedCardsAlwaysShown` | Ungated card present for alice, bob, and NopACL. |
| 2 | `TestDashboardPermission_HolderSeesGatedCard` / `_NonHolderFiltered` | alice sees the gated card; bob's response omits it and keeps the ungated ones, in order. |
| 3 | `TestDashboardPermission_NopACLShowsEverything`, `_ReadOnlyShowsEverything` (subtests "value form" / "face form") | Gated cards render under both no-policy ACLs. |
| 3 | `TestDashboardPermission_ReadOnlyArmIsExplicit` | **The RR-CWWJGW canary.** Attach a gate denying *every* permission, assert read-only still shows the card. Fails if the explicit arm is removed. Mutation-verify both directions. |
| 4 | `TestDashboardPermission_NilACLHides` | `aclImpl = nil` ⇒ gated card hidden, ungated unaffected. |
| 5 | `TestDashboardPermission_FilterIsPresentationOnly` | bob (data-permitted, permission-denied): card hidden **yet `_search` for that query returns rows > 0**. carol (permission-held, data-denied): card shown **yet `_search` returns 0 rows**. The row counts are the assertion, not the status code. |
| 6 | `TestDashboardPermission_ConfigUnfiltered` | `/_config` bodies byte-identical for alice vs bob, `dashboard:` included. |
| 7 | `TestDashboardPermission_EmptyCardsIsAlways200` | Three cases — no `dashboard:`, `cards: []`, all-filtered — each 200 with JSON `[]` (assert on the raw body, so `null` fails). |

**Layer 2 — SPA wiring** (`frontend/src/views/DashboardView.test.ts`, NEW — no
`DashboardView.test.ts` exists today). Mocks the `api/dashboard.ts` client and
asserts: cards render from the *endpoint response*; the empty state renders on
`cards: []`; the loading state covers the config fetch so "no cards" never
flashes before data arrives (RR-TIO1XP). This proves the SPA reads the new
endpoint, which no E2E currently can.

**Layer 3 — E2E regression only.** Keep `e2e/tests/dashboard.spec.ts` green: the
default ungated project must still render its cards after the endpoint swap
(`getCardCount() >= 1`, `dashboard.page.ts:28`). No gated-card E2E is attempted
— the suite has no multi-principal fixture, only a `--read-only` process-flag
spawn (`read-only-mode.spec.ts:91`), and building one is out of scope per
RR-PZKLVV.

**Edge Cases:**

- No `dashboard:` configured ⇒ 200 + `[]` (was 404 in the earlier draft;
changed per RR-CLZB5I).
- `dashboard:` present with `cards: []` ⇒ 200 + `[]`.
- Every card filtered ⇒ 200 + `[]`. Deliberately indistinguishable from the two
above: same rendering, no signal either way.
- Empty-string `permission:` ⇒ treated as ungated (the short-circuit), matching
nav semantics.
- Card **order** must be preserved; `cardData` is index-keyed so a dense,
order-stable slice matters.
- The hardcoded validation card renders regardless — out of scope, but assert
it is unaffected so the change doesn't silently move it.
- Unicode/whitespace in a permission name: no normalization, exact string match
— same as nav. Not a new hazard.

**Negative Tests:**

- Unknown `acl.ACL` implementation ⇒ card hidden (closed switch). Pinned with a
local stub type.
- nil `*acl.Declarative` ⇒ hidden, not fail-open.
- Non-GET method ⇒ 405, matching `handleV1Sidebar`.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Mitigation |
|---|---|
| **Rationale drift** — someone later treats card-hiding as confidentiality and builds on a secrecy property that was never real. This is the exact failure that got TKT-M1AX6P reverted. | Godoc says it plainly; AC5's two-directional test pins presentation-only; docs updated in the same change. |
| **Read-only arm copied wrongly** (the RR-XYO03L critical). | Sharing ONE predicate instead of copying the switch removes the opportunity entirely; the canary test fails if the arm is deleted. |
| **The extracted predicate becomes a general-purpose auth helper** (RR-QAEM5Z). | Name reads wrong off the presentation path (`permitsGatedUIElement`); godoc moves onto it; the grep guard keeps two needles rather than one widened allowlist. |
| **New endpoint = new surface.** | It serves config `/_config` already serves to everyone, minus entries — strictly less than the existing endpoint. No new data exposure. |
| **SPA/endpoint skew** — an older cached bundle reads `schemaStore.dashboard` and shows unfiltered cards. | Harmless by construction (UX filter, and `/_config` deliberately still carries the config). Noted, not defended against. |
| **Empty-state flicker** (RR-TIO1XP) — the all-filtered state and the loading state look identical. | `loading` covers the config fetch; pinned by the new `DashboardView.test.ts`. |
| **`permission:` typo ⇒ invisible card, no error** (RR-2KZEXF). | Out of scope to fix; documented as a named gotcha pointing at the roles' `permissions:` list, exactly as TKT-TXDK8U's docs did. |

**Effort:** s → **m**. Raised after design review: the SPA work is larger than
first estimated (new API client, a store-cached fetch, corrected loading state,
and a brand-new `DashboardView.test.ts`), and the endpoint swap touches the
existing E2E path. The Go side remains small.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/data-entry.md` — add `permission` to the dashboard card options
table (~line 1270-1330), with the "not a security boundary" note and the
RR-2KZEXF typo gotcha.
- [x] `docs/acl-security.md` — extend the sidebar-filtering section (~579-608,
"buys tidiness, not protection") to cover dashboard cards under the same
rationale.
- [x] `internal/dataentry/CLAUDE.md` — extend "Sidebar navigation filtering"
to cover the shared predicate and the new endpoint; state that `/_config` is
still not filtered and that the dashboard endpoint is the per-principal seam.
Fix the stale `TestNavPermission_ReadOnlyHides` reference while there.
- [x] ~~`docs/metamodel.md`~~ (N/A: this is a `data-entry.yaml` key, not a metamodel feature)
- [x] ~~`docs/cli-reference.md`~~ (N/A: no command changes)
- [x] ~~`README.md`~~ (N/A: no project-level change)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

- **RR-PZKLVV** (significant) — E2E ACL proof layer assumed a multi-principal
fixture that does not exist. *Addressed*: Test Plan restructured into three
layers, each claiming only what it can prove; E2E demoted to regression; the
fixture is explicitly out of scope.
- **RR-QAEM5Z** (significant) — extracting the shared predicate weakens the
grep guard. *Addressed*: renamed to `permitsGatedUIElement`, godoc moves to the
extracted function, guard keeps two needles; the earlier misreading of the
test's doc is corrected.
- **RR-CLZB5I** (minor) — 404-when-no-dashboard contradicted AC7 and had no
precedent in `handleV1Sidebar`. *Addressed*: always 200 with `[]`; AC7 and the
edge cases rewritten.
- **RR-TIO1XP** (minor) — added round-trip and empty-state flicker.
*Addressed*: store-level caching and loading-state coverage are now explicit
decisions with a test.
