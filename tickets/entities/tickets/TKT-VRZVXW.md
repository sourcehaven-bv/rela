---
id: TKT-VRZVXW
type: ticket
title: 't.Parallel wave: lua, mcp, write-path packages + -shuffle=on in CI'
kind: test
priority: medium
effort: s
status: done
---

## Problem

`t.Parallel()` appears in 76 of ~2,400 test functions (~3%). Zero in
`internal/lua` (largest suite, isolated runtime per test), zero in the
write-path cluster, zero in mcp (which builds a fresh bleve index per test, ~200
times, serially). Wasted wall-clock and unexercised race detection — CI runs
`-race` but serial tests barely stress it, despite an architecture explicitly
built for parallel tests (fresh store/app per test, MemFS).

Inter-test ordering dependencies are also undetected: tests always run in source
order.

## Approach (agreed with reviewer in session)

1. Wave: lua → mcp → entitymanager/automation/autocascade → acl/affordances/validation. Add `t.Parallel()` to top-level test functions; table subtests untouched in this pass.
2. Deliberate exclusions: `lua/ai_test.go` tests using `t.Setenv` (runtime-refused in parallel tests), `validation/lua_timeout_test.go` wall-clock latency tests (stay serial, commented), dataentry/cli (await fixture consolidation), storage watcher tests (real fsnotify).
3. `-shuffle=on` added to CI test job and justfile test recipes (failures print the seed for reproduction).
4. `paralleltest` linter NOT enabled — it's a per-package ratchet for after broader conversion.
5. One commit per package so flakes bisect cleanly. Each package verified with `go test -race -count=2 -shuffle=on` locally.

## Verification

- Per-package: `-race -count=2 -shuffle=on` green locally.
- Full `just ci` green; CI green on PR.
