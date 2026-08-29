---
id: TKT-EG33Y1
type: ticket
title: 'Extend the icon set to a curated ~150 names, generate registry + docs from one source, add icon: none'
kind: enhancement
priority: medium
effort: m
status: done
---

Grow the data-entry icon allowlist from 16 to a curated ~120–180 lucide names,
make a single canonical definition generate the Go allowlist, the SPA registry
and an end-user docs table, and add `icon: none` so a nav item can deliberately
have no glyph.

## Problem

Three distinct problems, one shared root — the icon set is defined twice by hand
and documented nowhere.

**1. The set is too small.** `frontend/src/utils/icons.ts` ships 16 names
(`dashboard`, `list`, `kanban`, `search`, `calendar`, `warning`, `apps`,
`settings`, `document`, `sun`, `moon`, `inbox`, `wrench`, `done`, `clock`,
`status`). rela is a general entity-graph platform — projects model ISMS
controls, runbooks, people, contracts, assets. A sidebar for any of those hits
the wall immediately, and the author's only escape is an emoji in `label:`,
which is exactly what TKT-8GUI60 set out to eliminate (emoji can't take the
theme colour and render differently per OS).

**2. The names are undiscoverable.** `docs/data-entry.md` deliberately does
*not* list them. That was the right call at the time — RR-GTOQCF found the prose
list had already gone stale within one ticket, so it was replaced by a pointer
to the startup error message. But "trigger a config error to find out what
you're allowed to write" is a poor discovery path, and it gets worse in
proportion to the set size. The fix RR-GTOQCF actually points at is
*generation*, which was out of scope then and is the core of this ticket.

**3. No way to opt out of an icon.** Every nav entry gets a kind-derived icon
(`list:` → list glyph, `kanban:` → board glyph), and `icon:` can only *override*
it, never suppress it. Apple's HIG is explicit that menu icons should be used
["sparingly and with
purpose"](https://developer.apple.com/design/human-interface-guidelines/menus#Icons):
a menu where every row carries a glyph conveys no more than one where none do,
and the current derived-icon default forces precisely that. An author who wants
one accented entry in a plain list cannot express it.

## Scope

**In scope**

- A canonical icon definition (name → lucide component, category, description)
in `internal/dataentryconfig`, expanded to ~120–180 curated names.
- A generator producing, from that one definition:
  - the Go allowlist (`ValidIconNames`),
  - the SPA registry (`frontend/src/utils/icons.ts` imports + `ICONS` map),
  - a categorised markdown table in `docs/data-entry.md`.
- `icon: none` as a reserved name suppressing both the authored and the
kind-derived icon, on navigation items (and kanban columns/swimlanes for
consistency).
- Sidebar layout: a no-icon item **reserves** the icon column so labels stay
aligned with icon-bearing siblings.
- CI drift check (`just docs-check`-style) so a hand-edit of a generated file
fails rather than silently diverges.

**Out of scope**

- Exposing all ~1600 lucide icons. Static imports are a deliberate security
property (`resolveIcon` must never resolve an arbitrary component from a config
string) and a bundle-size one; a curated set keeps both.
- Custom/uploaded SVG icons.
- Icons anywhere new (list columns, form fields, buttons) — this ticket
extends and documents the existing surfaces only.
- A group-level or global "icons off" switch. Per-item `icon: none` is the
primitive; a coarser control can be layered later if wanted.
- Restyling or re-theming the sidebar beyond the reserved-slot rule.

## Design decisions (settled with the requester)

| Decision | Choice | Rationale |
| --- | --- | --- |
| Source of truth | Canonical table in Go, generating TS + docs | The Go side already validates author input and must have the list anyway; generating outward beats parsing TS with a regex (which the current parity test does, fragilely) |
| No-icon syntax | `icon: none` | `icon: ""` already means "use the derived icon" — reusing it would silently change existing configs' meaning |
| No-icon layout | Reserve the icon column | Labels stay aligned in a mixed menu; collapsing looks ragged |
| Set size | ~120–180 curated | Covers realistic domains, stays a browsable table, keeps static imports |

## Acceptance criteria

1. **Single source.** Exactly one file defines the icon set. Adding an icon
there and running the generator updates the Go allowlist, the SPA registry and
the docs table with no other hand edit.
2. **Drift fails CI.** Hand-editing a generated file (or adding an icon without
regenerating) fails a check with a message naming the generator command.
3. **Set size.** The allowlist contains ≥120 names, organised into categories,
and every one resolves to a real lucide component that renders as inline SVG
with `stroke="currentColor"`.
4. **Docs table.** `docs/data-entry.md` contains a generated, categorised table
of every valid name with a short description, in a marked region, reachable from
both the kanban-icon and nav-icon sections.
5. **`icon: none` on nav.** A navigation item with `icon: none` renders with no
glyph, and its label aligns with icon-bearing siblings in the same group.
6. **`none` beats derivation.** `icon: none` on a `list:`/`kanban:`/`calendar:`
/`dashboard`/`search`/`settings`/`document:` entry suppresses the kind-derived
icon, not just an authored one — and reaches the SPA as the literal `"none"`,
not as an empty string (empty is dropped by `omitempty` and is also what a
malformed entry yields, so asserting emptiness would certify nothing). When the
sidebar is collapsed the derived glyph returns, since a row with neither icon
nor label is invisible yet still clickable.
7. **`none` validates.** `icon: none` passes config validation; `icon: nOnE` or
any other unknown name still fails at load with an error listing valid names.
8. **No regression.** All 16 existing names keep their current meaning and
component — a config authored today renders identically.
9. **Security property intact.** `resolveIcon` still resolves only via a
static own-property allowlist lookup; no dynamic component resolution from a
config string, and prototype keys (`toString`, `constructor`) still fall back.
