---
id: TKT-1WV50C
type: ticket
title: 'visibility: name the ungated read path (visibility.Unrestricted) so every bypass is greppable'
kind: refactor
priority: medium
effort: s
status: backlog
---

## Summary

Follow-up to TKT-ZF2DTV (RR-R0G3DF). `lua.EntityReader` is satisfied
**structurally** by `store.Store`, so an ungated wiring and a gated one are
indistinguishable when reading a struct literal — you have to read the
right-hand expression to tell. That is what allowed three production wiring
sites to be silently ungated during review.

## Proposal

A named capability wrapper, mirroring the existing `visibility.AllowAllReader`
idiom:

```go
// Unrestricted wraps a raw store as a script read handle. Named so that
// `grep -r Unrestricted` enumerates every ungated read path in one command.
func Unrestricted(st store.Store) lua.EntityReader
```

Then:
- CLI, docs runtime and the validator pass `visibility.Unrestricted(st)` instead of the bare store.
- `appbuild.scriptEntityReader` / `dataentry.App.scriptReader` return it on the NopACL path (and, where they still degrade, on the fallback path).
- The type system still permits a bare store — this is about legibility and auditability, not enforcement.

Same trick already used one level down for `WritePrepStore`: make the unsafe
choice *look* unsafe at the call site.

## Why not in TKT-ZF2DTV

Cross-cutting rename touching every wiring site plus the new wiring tests, mixed
into an already-large security change. The concrete risk it mitigates is now
covered by the black-box wiring tests (`luawiring_test.go` in dataentry and
appbuild), which fail if any site reverts to the raw store. This is the
ergonomic follow-through, not the safety net.

## References

- RR-R0G3DF, TKT-ZF2DTV
- `internal/visibility/{luareader.go, allowall.go}`, `internal/appbuild/appbuild.go`, `internal/dataentry/app.go`
