---
id: BUG-K3T3SR
type: bug
title: Data race on Engine.meta between evaluator and SetMetamodel
priority: low
status: backlog
---

## Symptom

`go test -race` reports a data race between `Engine.evaluator()` and
`Engine.SetMetamodel`:

```text
WARNING: DATA RACE
Read at ...: (*Engine).evaluator()      engine.go:38
Previous write at ...: (*Engine).SetMetamodel() engine.go:133
```

## Root cause

`evaluator()` reads `e.meta` **before** taking `evMu`:

```go
func (e *Engine) evaluator() *predicatefns.Evaluator {
	if e.meta == nil {        // <- unsynchronized read
		return nil
	}
	e.evMu.Lock()
	defer e.evMu.Unlock()
	...
}
```

and `SetMetamodel` writes `e.meta` outside the lock, guarding only the `e.ev`
reset:

```go
func (e *Engine) SetMetamodel(meta *metamodel.Metamodel) {
	e.meta = meta            // <- unsynchronized write
	e.evMu.Lock()
	e.ev = nil
	e.evMu.Unlock()
}
```

## Why it is currently latent

`SetMetamodel` has **no production callers** — engines are built via
`NewEngineFromMetamodel`, which sets `e.meta` once before the engine is shared.
So no live code path writes `e.meta` concurrently with a read.

It is filed rather than ignored because `evaluator()` became load-bearing on the
write path when automation conditions moved onto the predicate engine
(TKT-J4IR1G, TKT-8GD41J). Anyone wiring metamodel hot-reload — the obvious
reason `SetMetamodel` exists — would turn a latent race into a live one on a
path that now runs for every write.

## Fix

Guard both sides with the existing mutex: move the nil check inside the lock in
`evaluator()`, and take `evMu` around the `e.meta` write in `SetMetamodel`.
Cheap — the lock is already there and uncontended.

## Verification

Reproduce by calling `SetMetamodel` concurrently with `Process` under `-race`.
Note the current suite is `-race` clean (verified with 50 concurrent `Process`
calls against a condition-bearing automation) precisely because nothing calls
`SetMetamodel` concurrently; the regression test has to create that situation
deliberately.
