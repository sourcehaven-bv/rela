---
id: IMPL-R9OI
type: implementation-checklist
title: 'Implementation: Migrate dataentry server to wire its own services (off Workspace)'
status: done
---

## Implementation

- [x] NEW `internal/appbuild` package: `Services` (method-only accessors), `Discover()`, `New()`, `Close()` (idempotent via sync.Once), plus `LuaReadDeps()` and `LuaWriteDeps()` helpers.
- [x] `cmd/rela-server/main.go`: `workspace.Discover` → `appbuild.Discover`. Scheduler boot passes `*appbuild.Services` (satisfies `scheduler.WorkspaceProvider` structurally).
- [x] `cmd/rela-desktop/main.go`: `workspace.New` → `appbuild.New`. Per-project lifecycle now closes the previous svc on every LoadProject (was leaking goroutines + bleve indexes).
- [x] `internal/dataentry/e2e_test.go`: workspace → appbuild migration so dataentry's e2e build no longer drags workspace.
- [x] `.go-arch-lint.yml`: new `appbuild` component; cmdServer/cmdDesktop dependency lists tightened (dropped workspace).
- [x] `internal/appbuild/appbuild_test.go`: covers Discover, LuaReadDeps/LuaWriteDeps derivation, missing-project error, nil-deps rejection, and Close-after-Close idempotency.
- [x] `just ci` green; `go test -tags=e2e` builds clean.

## Cranky review disposition

| # | Severity | Status | Notes |
|---|----------|--------|-------|
| 1 | critical | **Addressed** | Desktop now tracks `svc *appbuild.Services` on the struct and closes the previous one in sequence (scheduler stop → unlock → close) on every `LoadProject`. Previously every project switch leaked the whole service stack. |
| 2 | critical | **Addressed** | Dropped `Services.ScriptEngine()` — no production consumer. The scheduler still mints its own engine (pre-existing inefficiency, not a regression here). |
| 3 | critical | **Addressed** | `e2e_test.go` migrated to appbuild. |
| 4 | significant | **Addressed** | `nopKV.Get` returns `os.ErrNotExist` (so scheduler's "no last run yet" semantics work). Put/Delete silently no-op. Doc comment explains the deliberate divergence from workspace.nopState. |
| 5 | significant | **Addressed** | `Close()` wrapped in `sync.Once` + records `closeErr`. Safe to call from multiple goroutines, idempotent across calls. Tested. |
| 6 | significant | **Addressed** | `buildStateKV` returns `(state.KV, error)` and `New` propagates. Desktop's `LoadProject` surfaces it to the UI instead of crashing the host. |
| 7 | significant | Acknowledged | LuaWriteDeps assignment lives on entitymanager.EntityManager — narrow consumer-side typing is TKT-IF37's territory, not this PR's. |
| 8 | minor | **Addressed** | rela-server's "No defer svc.Close()" comment now explains daemon-lifetime choice and contrasts with desktop's per-project close. |
| 9 | minor | Won't fix | `backfillSearchBackend` duplicated from workspace — acceptable while workspace exists. Will consolidate when TKT-64R3 deletes workspace. |
| 10 | minor | **Addressed** | Added `TestClose_Idempotent` — three sequential Closes, all return nil. |
| 11 | minor | Won't fix | `DiscoverDefault` (engine-less Discover) — not enough callers to justify, sticking with single Discover signature. |
| 12 | leverage | Won't fix | Helper-extraction inside `New` — speculative until 3rd entry point appears. |
| 13 | leverage | Partially addressed | sync.Once handles re-close; post-close state-after-use isn't pinned by a test, but a misbehaving observer past Close hitting a closed store is bleve's failure mode, not ours. |
