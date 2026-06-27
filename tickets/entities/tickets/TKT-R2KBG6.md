---
id: TKT-R2KBG6
type: ticket
title: 'Fixture consolidation: mcp on appbuildtest, validation metamodel dedup, testutil fixes'
kind: test
priority: medium
effort: s
status: done
---

## Problem

Test-fixture sprawl identified in the test-quality review:

1. `internal/mcp/test_helpers_test.go` `newTestDeps` hand-duplicates ~75 lines of production service wiring (tracer, bleve index + backfill, validator, autocascade, entitymanager) with a self-acknowledged drift risk. The nil-templater booby trap found by TKT-TLQ94B lived in exactly this hand-rolled wiring. `appbuildtest` exists to prevent this and is already used by dataentry/cli.
2. `validation/lua_test.go` inlines a ~30-line metamodel + entity literal ~20 times; a schema-shape change touches all of them. `internal/testutil` has metamodel-aware builders that nobody there uses.
3. `testutil` itself has sharp edges: `AssertEqual` compares `interface{}` with `!=` (panics on uncomparable types, weak failure output); `AssertStringContains` hand-rolls a byte loop instead of `strings.Contains`.

## Approach (agreed with reviewer in session)

1. Port `mcp.newTestDeps` to `appbuildtest.New(meta, WithStore(st))` — Services exposes everything mcp.Deps needs incl. `LuaWriteDeps()`; bleve + backfill semantics match the current fixture. Keep the package-local `nopWatcher` (consumer-side stub, idiomatic).
2. Do NOT merge the three `mockWorkspace` types (consumer-side stubs stay per CLAUDE.md); deduplicate the shared metamodel + seeded-store construction instead.
3. Demonstration migration: `validation/lua_test.go` inline metamodel literals → one canned fixture + per-test deltas via testutil builders.
4. Fix testutil sharp edges (`reflect.DeepEqual`, `strings.Contains`).

Dropped from the original proposal at reviewer request: the CLAUDE.md
test-conventions update (direction is a future all-stdlib rewrite; don't
enshrine a mixed convention).

## Verification

- mcp + validation + testutil packages green under `-race -count=2 -shuffle=on`; full `just ci` green; CI green on PR.
- The mcp port must not weaken the dispatch tests (TKT-TLQ94B) — they keep building via NewServer.
