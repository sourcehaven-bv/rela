---
id: IMPL-RJNO0C
type: implementation-checklist
title: 'Implementation: visibility.Unrestricted'
status: done
---

## Development

- [x] `visibility.Unrestricted(store.Store) *UnrestrictedReader` — named pass-through, three read methods only.
- [x] Returns a concrete type, not `lua.EntityReader`: `internal/visibility` must not import `internal/lua` (arch-lint), and structural satisfaction is the existing design.
- [x] Panics on a nil store (see RR-5QQL1Z — returning a typed nil bypasses lua's deny guard and nil-derefs).
- [x] Six sites converted, including both NopACL branches (RR-65GNYB).
- [x] `.go-arch-lint.yml`: added `visibility` to `docs` and `appbuildtest` `mayDependOn`, each with a scoping comment naming the ticket.

## Quality

- [x] `go build ./...` clean
- [x] `just lint` — 0 issues
- [x] `just arch-lint` — no warnings (the load-bearing check: proves no lua import crept in)
- [x] `just plimsoll` — pass
- [x] Full `./internal/...` suite passes except `docscapture` (pre-existing: `frontend/dist/` not built locally)
- [x] Every test mutation-verified: adding a write method, adding a redaction, reverting the nil panic, and substituting DenyReader on the NopACL path each fail the corresponding test.
- [x] ~~Behavior change~~ (N/A by design: pure pass-through; the one intended behavior change is the nil panic, which replaces an unreachable-but-wrong path)
