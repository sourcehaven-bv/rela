---
id: TKT-1WV50C
type: ticket
title: 'visibility: name the ungated read path (visibility.Unrestricted) so every bypass is greppable'
kind: refactor
priority: medium
effort: s
status: done
---

## Summary

Follow-up to TKT-ZF2DTV (RR-R0G3DF). `lua.EntityReader` is satisfied
**structurally** by `store.Store`, so an ungated wiring and a gated one are
indistinguishable when reading a struct literal — you have to read the
right-hand expression to tell. That is what allowed three production wiring
sites to be silently ungated during review.

## Design correction: return type

The original sketch proposed:

```go
func Unrestricted(st store.Store) lua.EntityReader
```

That signature is **wrong for this package**. `internal/visibility` deliberately
does not import `internal/lua` — it satisfies `lua.EntityReader` *structurally*
(see the note in `luareader.go:13`), and `.go-arch-lint.yml:521-528` allows
visibility to depend only on acl/affordances/entity/principal/store/tracer.
Importing lua would fail `just arch-lint`.

So `Unrestricted` returns a named concrete type whose method set matches
`lua.EntityReader`. Structural satisfaction keeps working, and the wiring sites
still read `visibility.Unrestricted(st)`.

## Sites (enumerated, not assumed)

`grep -rn "VisibleReader:"` gives four, three of them production:

| Site | Why it is ungated |
|---|---|
| `appbuild.go:200` (`LuaReadDeps`) | Operator trust boundary — CLI/docs; whoever runs the binary has the project files (RR-17DMC) |
| `dataentry/app.go:540` (validator deps) | Redacting here manufactures false violations: a rule asserting "every ticket links to a project" would fire on projects the principal cannot see |
| `docs/runtime.go:159` | Throwaway memstore the doc build just seeded itself |
| `appbuildtest/fixture.go:298` | Test fixture |

Each is deliberate and already carries a prose justification. This ticket makes
that decision **greppable**, it does not change behavior.

## Approach

- Add `visibility.Unrestricted(store.Store)` returning a named type
(`UnrestrictedReader`) that delegates all three methods to the store.
- Convert the three production sites plus the test fixture.
- Keep every existing justification comment; the wrapper names the choice, the
comment still explains it.
- Non-goal: enforcement. The type system still permits a bare store — this is
legibility and auditability. `grep -rn "visibility.Unrestricted"` must enumerate
every ungated read path in one command.

## Acceptance criteria

- `grep -rn "visibility.Unrestricted" --include=*.go` lists every ungated
script read path, and no `VisibleReader:` line in production is a bare store.
- Behavior byte-identical: reads pass straight through, no copying, no gating.
- `just arch-lint` passes (proving no lua import crept in).
- A test pins that the wrapper is a true pass-through and that it is NOT a
`store.Store` (so an accidental re-widening is visible).

## Why not in TKT-ZF2DTV

Cross-cutting rename touching every wiring site plus the new wiring tests, mixed
into an already-large security change. The concrete risk it mitigates is now
covered by the black-box wiring tests (`luawiring_test.go` in dataentry and
appbuild), which fail if any site reverts to the raw store. This is the
ergonomic follow-through, not the safety net.

## References

- RR-R0G3DF, TKT-ZF2DTV
- `internal/visibility/{luareader.go, allowall.go}`, `internal/appbuild/appbuild.go`, `internal/dataentry/app.go`
