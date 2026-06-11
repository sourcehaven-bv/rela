---
id: PLAN-NASDVM
type: planning-checklist
title: 'Planning: Fixture consolidation'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined — see TKT-R2KBG6 body; CLAUDE.md update dropped at reviewer request
- [x] Acceptance criteria: (1) mcp test wiring single-sourced via appbuildtest, no hand-rolled service graph remains; (2) validation/lua_test.go has one metamodel fixture instead of ~20 literals, tests semantically unchanged; (3) testutil.AssertEqual handles uncomparable types via reflect.DeepEqual, AssertStringContains uses strings.Contains; (4) all affected packages green under -race -count=2 -shuffle=on

## Research

- [x] ~~/research~~ (N/A: test-only consolidation)
- [x] Verified appbuildtest covers mcp's needs before committing to the approach: in-memory bleve + backfill for caller-supplied stores (fixture.go:186-194), `Services.LuaWriteDeps()` accessor exists, templater is a real FSTemplater over the fixture FS (strictly better than the nopTemplater workaround)
- [x] Checked the WithStore caveat (no observer auto-wiring → post-construction writes don't reach the index) — matches the current mcp fixture's behavior exactly, so no test semantics change

## Approach

- [x] Replace newTestDeps body with appbuildtest.New + accessor mapping; keep nopWatcher stub; keep Deps.ProjectRoot = t.TempDir() (lua_run path semantics)
- [x] validation/lua_test.go: one package fixture (testutil.MetamodelBuilder + EntityBuilder) + per-test deltas
- [x] testutil fixes with their existing self-tests extended
- [x] Alternatives: merging mockWorkspace types rejected (consumer-side stubs per CLAUDE.md); adding observer wiring to WithStore rejected (out of scope, current semantics preserved)

## Security Considerations

- [x] N/A — test-only

## Test Plan

- [x] -race -count=2 -shuffle=on on mcp, validation, testutil; full just ci
- [x] Dispatch tests (TKT-TLQ94B) must stay on NewServer — they are the canary that the ported wiring still satisfies NewServer's nil checks

## Risk Assessment

- [x] Effort s. Risks: config loader behavior delta (nopConfigLoader errors vs FSLoader not-found — both error; verify no test pins the nop message); search-index timing differences (none expected, same backfill shape)

## Documentation Planning

- [x] N/A (test-only)

## Design Review

- [x] ~~/design-review~~ (N/A-with-substitute: approach discussed and amended in working session 2026-06-10 — CLAUDE.md item dropped, mockWorkspace merge explicitly rejected)
