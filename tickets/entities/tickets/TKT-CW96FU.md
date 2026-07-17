---
id: TKT-CW96FU
type: ticket
title: Remove the `rela create --title` write shortcut
kind: refactor
priority: medium
effort: s
status: done
---

## Problem

`rela create <type> -t/--title "X"` writes `X` into whatever
`EntityDef.GetPrimaryProperty()` resolves to (falling back to `"title"` when
that's empty). This conflates two distinct concerns:

- **Display** — which property renders as the entity's human-readable name
(readonly, derived).
- **Write target** — which property a create shortcut should set.

`internal/cli/create.go` repurposed the *display* property as a *writable*
target. That was always fragile, and it becomes incoherent once
`display_property` can be a multi-property template (`"{voornaam}
{achternaam}"`) — there is no single field to write a `--title` value into. See
TKT-2SVA3L (display_property templates), which surfaced this.

## Decision

Rip out `--title` entirely. No backwards-compat shim. Users set the title (or
any property) with the existing, unambiguous `-P/--property title="X"` flag,
which already works for every property.

## Changes

- `internal/cli/create.go`: remove the `Title` field / `-t` flag from
`CreateCmd` and the block that writes it into `GetPrimaryProperty()`'s target.
`GetPrimaryProperty()` stays (still used readonly by `DisplayTitle`, `mentions`,
`affordances`, and the `Primary` wire hint).
- Docs: migrate every `rela create <type> --title "X"` example to
`rela create <type> -P title="X"` and drop the `-t, --title` row from the CLI
reference. Source of truth is `docs-project/entities/guides/GUIDE-*` + `TUT-*`
(6 files, ~214 example lines); `docs/*` regenerates via `just docs`.

## Acceptance criteria

1. `CreateCmd` has no `Title`/`-t` flag; `rela create requirement -t x` errors as an unknown flag.
2. `rela create requirement -P title="X"` still sets the title (unchanged path).
3. `go build ./...` and `just test` pass (no test exercised `--title`).
4. `just docs` regenerates cleanly; `docs-check` passes with no `--title` references remaining.

## Out of scope

- The display_property template feature itself (TKT-2SVA3L).
- Any replacement "smart title" flag — deliberately none; `-P` is sufficient.
