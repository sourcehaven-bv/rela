---
id: TKT-64R3
type: ticket
title: Delete internal/workspace once all consumers have migrated
kind: refactor
priority: low
effort: s
status: ready
---

Final step of the workspace-decomposition arc: delete `internal/workspace`
entirely.

**Preconditions (verified before PR):**

- `grep -rn 'internal/workspace' --include="*.go" .` returns zero hits outside `internal/workspace/` itself
- Production CLI moved to `*appbuild.Services` (TKT-DS43)
- `internal/dataentry` test helpers (`test_helpers_test.go`, `watcher_test.go`) migrated to `appbuild.NewForTest` as part of this PR

**Changes:**

- `git rm -r internal/workspace/`
- Migrate `internal/dataentry/test_helpers_test.go` and `internal/dataentry/watcher_test.go` from `workspace.NewForTest` → `appbuild.NewForTest` (same `WithFS` shape)
- Fix `appbuild.NewForTest` to build a real `state.KV` when FS+CacheDir are supplied (mirrors workspace semantics; dataentry UIState/UserDefaults tests need this)
- Extract helpers from `NewForTest` to fix funlen
- Clean up stale `internal/workspace` godoc references in `analysis`, `attachment`, `entitymanager`, `lua/scripterror`, `renametype` (no behavior change)
- `.go-arch-lint.yml`: remove `workspace` component declaration and all `mayDependOn: workspace` entries

**Scope:** ~3500 LOC deleted; ~50 LOC test-helper migration + ~50 LOC appbuild
test-fixture refinement.

After this lands, `internal/workspace/` is gone; every entry point constructs
its own narrow service bundle.
