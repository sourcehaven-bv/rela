---
id: TKT-3D483X
type: ticket
title: 'dataentry: a context without a stamped world is an error, not the trivial scope'
kind: chore
status: backlog
---

## Description

`worldFromContext` (internal/dataentry/world.go) falls back to
`defaultWorldHandle()` when no world is stamped on the context. That handle is
the trivial scope: every id resolves to its implicit face. A faced type has no
implicit face (DEC-NPZICR), so under the fallback its entities look missing.

Every API request has a world stamped by `attachWorld`, so the fallback is only
reached by code outside an API request: non-API routes, background callers and
tests. There it hides a wiring mistake as a silent "not found" instead of
surfacing it.

### Proposed

- Make an unstamped context an error rather than the trivial scope, for
example by returning `store.ErrInvalidQuery` from the world lookup (the same
answer an unset `visibility.World` already gives), so the failure is loud.
- Stamp an explicit world at every non-request caller that needs one.
- Fix the tests that rely on the fallback by stamping a world in their context.
