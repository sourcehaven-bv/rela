---
id: TKT-53KICM
type: ticket
title: 'Permission-based dashboard card filtering (UX: hide cards a user cannot use)'
kind: enhancement
priority: medium
effort: m
status: review
---

## Goal

Let an operator put an optional `permission:` on a **dashboard card** and have
cards the principal does not hold the permission for omitted, exactly as
`permitsNavEntry` already does for sidebar entries (TKT-TXDK8U). Requested by
the user: "filter dashboard cards similarly to nav items (so for UX purposes)
via a permission".

## Framing (read before implementing)

This is the **same framing as TKT-TXDK8U**, and it must not drift:

- **UX filter, NOTHING ELSE.** A hidden card conceals nothing. The card's query
runs through `/api/v1/_search`, which is already ACL-scoped (`helpers.go:411`,
`SearchScope`) — a principal who may read none of the matching entities gets an
empty result today. Hiding the card only stops rendering a tile that would read
`0` or blank.
- **Do not describe this as concealment** in code, docs, or tests, and do not
add tests asserting a card title or query is unenumerable. Config is not a
secret (root `CLAUDE.md`, "The configuration is not a secret; the data is").
- The endpoints behind every card keep enforcing exactly what they enforce
today. Nothing downstream may rely on a card being hidden.

## Design decisions (settled in planning — see PLAN-X3SW5Y)

The non-obvious problem: dashboard cards are served **only** through `/_config`
(`api_v1.go:1338`), which `internal/dataentry/CLAUDE.md` forbids filtering, and
`TestNavPermission_ConfigUnfiltered` pins byte-identical across principals.
Unlike the sidebar, there was no per-principal endpoint to filter into.

Resolved with the user:

1. **A new `GET /api/v1/_dashboard` endpoint** serves the resolved, filtered
card list — the `/_sidebar`-shaped answer. `/_config` keeps serving `dashboard:`
verbatim and the pinned test stays green. The two rejected alternatives (a
per-card `permitted` flag on `/_config`, or filtering `/_config` outright) both
make `/_config` principal-dependent, which is the concealment-shaped direction
already reverted once in TKT-M1AX6P.
2. **Hide the card entirely** rather than greying it out — matching nav-item
behaviour, which is what was asked for.
3. **Always 200 with `cards: []`** — never 404, never `null` (RR-CLZB5I).
4. **One shared ACL switch**, extracted from `permitsNavEntry` as
`permitsGatedUIElement`, so the RR-XYO03L read-only arm cannot drift between the
two surfaces (RR-QAEM5Z).

## Scope

**In scope**
- `permission:` on `DashboardCard` (`dataentryconfig/config.go:625-634`).
- The `/api/v1/_dashboard` endpoint + wire types.
- The extracted shared predicate + the `lint_test.go` grep guard update.
- SPA: new API client, store-cached fetch, corrected loading state, empty state.
- Docs: `docs/data-entry.md`, `docs/acl-security.md`,
`internal/dataentry/CLAUDE.md`.

**Out of scope**
- Validating `permission:` values against `acl.yaml` — deferred for ALL
consumers in RR-2KZEXF; this adds a fourth to the same known gap. Documented,
not fixed here.
- Hiding cards on a zero ACL-scoped result count (declined in TKT-TXDK8U: it
makes the UI flicker as data changes).
- Hiding the "Dashboard" sidebar entry when all its cards are filtered.
- The hardcoded validation-summary card (`DashboardView.vue:206-234`) — not a
configured card, has no `permission:` to carry.
- A multi-principal E2E fixture (RR-PZKLVV) — none exists today; its own ticket.
- Any new `permission:` surface on `list:`/`kanban:`/`action:`.

## Acceptance criteria

1. A card with no `permission:` renders for everyone; short-circuits before any
ACL work.
2. Under `*acl.Declarative`, a card with `permission: x` renders only for a
principal holding `x`.
3. Under `NopACL` **and** `ReadOnlyACL` (value *and* pointer forms), gated cards
**render**. Both mean "no policy configured"; see RR-XYO03L, where copying
`authorizeCommand`'s read-only deny arm was the critical bug — it hid entries
from *everyone* based on a process-wide flag about writes.
4. A nil ACL, or an ACL implementation with no arm, **hides** the card (closed
by construction).
5. The filter is presentation-only: `_search` behind a hidden card's query
returns exactly what it returned before.
6. `/api/v1/_config` stays byte-identical across principals, `dashboard:`
included.
7. A dashboard with no surviving cards returns 200 with `cards: []` and the SPA
renders an empty state — never `null`, never 404.

## Prior art in-tree

- `permitsNavEntry` + its godoc, `internal/dataentry/views_handler.go:351-429` —
the policy, the read-only reasoning, and the closed switch. Reuse, don't copy.
- `handleV1Sidebar`, `views_handler.go:192-257` — the endpoint shape, including
`make([]T, 0)` at line 209 so the wire carries `[]` not `null`.
- `internal/dataentry/nav_permission_test.go` — the test shapes to mirror
(`ReadOnlyArmIsExplicit`, `FilterIsPresentationOnly`, `ConfigUnfiltered`).
- `internal/dataentry/CLAUDE.md` § "Sidebar navigation filtering".
- RR-XYO03L (critical, addressed), RR-CWWJGW, RR-2KZEXF (significant, deferred).

## Origin

User request, following the same UX rationale as TKT-TXDK8U.
