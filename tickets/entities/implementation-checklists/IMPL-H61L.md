---
id: IMPL-H61L
type: implementation-checklist
title: 'Implementation: Narrow lua.WriteDeps.EntityManager and autocascade.Mutator to the 5 methods lua actually calls'
status: done
---

## Implementation

- [x] New `lua.Mutator` interface (5 methods) — consumer-side, defined at the lua call site.
- [x] `lua.WriteDeps.EntityManager` field type narrowed from `entitymanager.EntityManager` (7) → `lua.Mutator` (5).
- [x] `autocascade.Mutator` narrowed from 7 → 5 methods; godoc updated; "carried for shape symmetry" comment removed (replaced with TKT-IF37 narrowing breadcrumb).
- [x] `internal/lua` drops `internal/entitymanager` import (runtime.go + deps.go both clean). Arch-lint updated: `lua.mayDependOn` no longer includes `entitymanager`.
- [x] `mockManager` in `internal/lua/runtime_test.go` shrunk to 5 methods; assertion is `_ Mutator = (*mockManager)(nil)`.
- [x] Symmetry compile-time assertion at `internal/script/luascriptrunner.go`: `var _ lua.Mutator = autocascade.Mutator(nil)`. The only place that imports both Mutator types; the only legal location to pin the structural equivalence the assignment depends on.
- [x] Assignment-site comment updated to explain the structural cross-cast.
- [x] `lua.Mutator` doc comment fixed (no longer names `entitymanager.Manager` — that package is no longer importable from lua).
- [x] `just ci` green; tests pass under `-race`; verified the symmetry assertion catches drift in either direction (added a fake method to each side independently, confirmed the build breaks).

## Cranky review disposition

| # | Severity | Status | Notes |
|---|----------|--------|-------|
| 1 | significant | **Addressed** | Added `var _ lua.Mutator = autocascade.Mutator(nil)` compile-time assertion at the script package boundary. Drift in either direction is caught immediately. |
| 2 | significant | **Addressed** | `lua.Mutator` doc no longer names `entitymanager.Manager` (a type lua can't import). Re-phrased to describe the contract; named the production satisfier abstractly. |
| 3 | nit | Won't fix | em-dash spacing in godoc — matches the surrounding code, no need to special-case. |
| 4 | minor | **Addressed** | Breadcrumb in `autocascade.Mutator` doc: "Narrowed from seven to five in TKT-IF37." |
| 5 | leverage | Acknowledged | Could deduplicate the 5-method signatures into `internal/entity` — not now. Filed as a mental note; if a fourth consumer of the shape appears, revisit. |

## Verification of the symmetry assertion

Drift test — added `FakeMethod()` to autocascade.Mutator only:

```
internal/script/luascriptrunner.go:128:21: cannot use autocascade.Mutator(nil) as lua.Mutator value
```

Drift test — added `FakeMethod()` to lua.Mutator only:

```
internal/script/luascriptrunner.go:84:18: autocascade.Mutator does not implement lua.Mutator (missing method FakeMethod)
internal/script/luascriptrunner.go:128:21: ... same ...
```

Both directions caught at the script boundary, not three frames deep at runtime.
