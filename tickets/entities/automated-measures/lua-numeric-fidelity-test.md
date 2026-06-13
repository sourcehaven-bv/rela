---
id: lua-numeric-fidelity-test
type: automated-measure
title: 'Test: Lua numeric type fidelity and whole-string sort'
description: 'Regression for BUG-C97E5C: asserts integral Lua numbers convert to int64 (up to 2^53; out-of-range stays float64), and luaValueToSortable treats a string as numeric only when it parses entirely (so ''1.2.0''/''3 blind mice'' sort lexicographically). Fails if conversion reverts to always-float64 or sortable reverts to Sscanf prefix parsing.'
kind: test
location: internal/lua/numeric_test.go (TestLuaNumberToGo_PreservesIntegers, TestLuaValueToGo_IntegerNotFloat, TestLuaValueToSortable_OnlyWholeStringsAreNumeric, TestSortEntries_VersionStringsAreLexicographic, TestLuaNumberToGo_OutOfInt64RangeStaysFloat)
status: active
---
