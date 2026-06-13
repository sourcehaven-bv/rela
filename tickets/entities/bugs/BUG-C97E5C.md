---
id: BUG-C97E5C
type: bug
title: 'Lua numeric handling: integers lost to float64, version strings mis-sorted'
description: Two latent numeric bugs in internal/lua/runtime.go. (1) luaValueToGo converted every Lua number to float64, so an integer value (entity ID, ticket number) lost its integer type on the Lua->Go boundary and re-serialized in trailing-.0 / exponential form; the reverse direction already preserved int/int64. (2) luaValueToSortable used fmt.Sscanf('%f'), which accepts a numeric *prefix*, so 'rela.sort_entities' treated version-like strings ('1.2.0') and 'numeric-prefix' strings ('3 blind mice') as their leading number (1, 3) and mis-sorted them. The sort also used a hand-rolled O(n^2) bubble sort.
priority: medium
why1: luaValueToGo had a single LNumber->float64 case, and luaValueToSortable used Sscanf which stops at the first non-numeric character.
why2: gopher-lua's number type is float64-backed, so 'just cast to float64' looked correct, and Sscanf looked like a convenient numeric-parse without considering trailing junk.
why3: The Lua<->Go conversion was written for the common case (round-tripping small floats/strings) without a round-trip test for integer-type fidelity or whole-string numeric parsing.
why4: No test compared a version-like string sort or asserted integer type survived the boundary, so both stayed latent.
why5: Numeric type fidelity across an embedded-language boundary has no single owner or convention; each conversion site decided ad hoc.
prevention: luaValueToGo now converts an integral, in-int64-range Lua number to int64 (preserving integer type up to 2^53, the float64 integer ceiling) and keeps non-integral/out-of-range values as float64. luaValueToSortable uses strconv.ParseFloat over the trimmed whole string, so only fully-numeric strings sort numerically. The bubble sort is replaced with sort.SliceStable. Unit tests pin integer preservation, whole-string-only numeric detection, and lexicographic version-string ordering; the sortable cases fail without the fix.
status: done
---

## Bug

Found in the 2026-06-09 backend review (Minor / Lua-AI-scheduler). Two latent
numeric bugs in `internal/lua/runtime.go`:

1. **Integers collapse to float64 on Lua→Go** (`luaValueToGo`). The `LNumber` case did `return float64(v)`, so an integer property (entity ID, ticket number, epoch value) lost its integer *type* crossing the boundary and could re-serialize as `42.0` or in exponential form. `GoToLuaValue` already preserves `int`/`int64`, so only the reverse leg was lossy.

2. **`luaValueToSortable` parses a numeric prefix, not the whole string** (`rela.sort_entities`). `fmt.Sscanf(s, "%f", &n)` succeeds on any string *starting* with a number, so `"1.2.0"` → `1` and `"3 blind mice"` → `3`. Version-like and prefix-numeric strings were sorted by their leading number instead of lexicographically. The sort itself was a hand-rolled O(n²) bubble sort.

## Fix (PR pending)

- `luaNumberToGo`: convert an integral, in-`int64`-range Lua number to `int64`; keep non-integral or out-of-range values as `float64`. This preserves integer type up to 2^53 (the float64 integer ceiling — beyond that gopher-lua's `LNumber` can't hold an integer faithfully anyway). No downstream code asserts `float64` on converted values; JSON marshals `int64` cleanly.
- `luaValueToSortable`: `strconv.ParseFloat(strings.TrimSpace(s), 64)` — numeric only when the *whole* trimmed string parses. `"1.2.0"`/`"3 blind mice"` now sort lexicographically.
- `sortEntries`: `sort.SliceStable` replaces the bubble sort (stable, O(n log n)).

## Tests

`internal/lua/numeric_test.go`: integer preservation (incl. 2^53 ceiling and
out-of-range→float fallback), whole-string-only numeric detection
(version/prefix/junk cases), and end-to-end lexicographic version-string
ordering through `sortEntries`. The sortable cases verified to **fail without
the fix**. Existing `TestSortEntities_*` (numeric, string, descending) still
pass.
