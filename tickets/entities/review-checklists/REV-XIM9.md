---
id: REV-XIM9
type: review-checklist
title: 'Review: Migrate dataentry server to wire its own services (off Workspace)'
status: done
---

## Code Review

- [x] cranky-code-reviewer run on the diff
- [x] 3 critical findings addressed (desktop leak, dead ScriptEngine API, e2e_test stragglers)
- [x] 4 significant findings addressed (Close idempotency, buildStateKV panic→error, nopKV semantics, defer doc)
- [x] 3 minor findings addressed; 2 won't-fix with reasoning
- [x] 1 leverage finding partially addressed
- [x] `go test -race ./...` clean
- [x] `go test -tags=e2e` builds clean
- [x] `just ci` green

## Disposition

See IMPL-R9OI for the full table.

**Headline:** cranky caught a real goroutine + bleve-index leak in
`cmd/rela-desktop` (every project switch leaked the previous service stack). Now
`LoadProject` stops the prior scheduler, swaps services, releases the lock, and
closes the previous `*Services` outside the lock. Idempotent `Close()` via
`sync.Once` makes that safe.

**Smaller wins from review:**

- `nopKV.Get` returns `os.ErrNotExist` (so scheduler "no last run yet" semantics work) instead of the workspace.nopState "loud sentinel" pattern.
- `buildStateKV` returns error instead of panicking — appbuild is per-project on a long-running host, not a process singleton like workspace.
- `Services.ScriptEngine()` accessor dropped — no production consumer used it; only the test referenced it.
- `e2e_test.go` migrated to appbuild for symmetry with cmd/* (so `internal/dataentry` carries no production-or-e2e workspace import).

**Pre-existing inefficiency not regressed:** scheduler still constructs its own
`script.NewEngine()` inside `StartBackground`, so each project has two engines
(Manager's + scheduler's). Same as workspace did. Out of scope for this PR.
