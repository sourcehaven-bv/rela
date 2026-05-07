---
id: RR-P30TC
type: review-response
title: TestMdEntityRefs reuses one runtime across subtests, leaks Lua globals
finding: All TestMdEntityRefs subtests share one rt and one Lua state. Each subtest mutates the global 'result'. If a subtest fails after setting result, a later assertion may quote stale data. Also Lua stack state could leak between subtests. Per-subtest setup is the standard pattern.
severity: minor
resolution: Refactored TestMdEntityRefs to use a per-subtest newEntityRefsRuntime helper. Each subtest gets a fresh runtime + workspace with no shared Lua globals.
status: addressed
---

# Finding

`TestMdEntityRefs` at `markdown_test.go:1623-1705` constructs one `rt` outside
the subtests and reuses it. Each subtest mutates the Lua global `result`. Test
isolation is leaky:

- A subtest failing after setting `result` causes a later assertion
failing on `result` to read stale data.
- Lua stack state could leak between subtests.

# Resolution

Move `rt` construction inside each subtest, or reset the relevant globals
(`rt.L.SetGlobal("result", lua.LNil)`) between subtests. Standard pattern is
per-subtest setup.
