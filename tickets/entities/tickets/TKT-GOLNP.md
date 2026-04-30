---
id: TKT-GOLNP
type: ticket
title: Extract stubEntityManager to shared test helper package
kind: refactor
priority: low
effort: s
status: review
---

## Problem

Two copies of the same `EntityManager` test stub exist:

- `internal/dataentry/document_script_test.go:28-54` (as `docTestStubEM`)
- `internal/script/executor_test.go:23-51` (as `stubEntityManager`)

Both panic on every method call ("not expected in this test"). If the
`entitymanager.EntityManager` interface grows a method, both files must be
updated in lockstep or the tests stop compiling.

## Scope

Extract to a new package `internal/entitymanager/entitymanagertest` (or similar)
with:

```go
// PanicOnUse is an EntityManager whose every method panics. Use it
// when a test's code path should never reach a mutation.
type PanicOnUse struct{}

func (PanicOnUse) CreateEntity(...) ...
func (PanicOnUse) UpdateEntity(...) ...
// ...
```

Replace both copies with imports of the shared type.

## Out of scope

Adding a more capable fake (e.g., a recorder). That's a separate design
conversation — build it when we actually need one.
