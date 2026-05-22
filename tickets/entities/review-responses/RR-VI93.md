---
id: RR-VI93
type: review-response
title: NumberExpr.Value is a string — int/float discrimination unspecified
finding: 'Plan treats numbers as typed int/float in the Value type, but gopher-lua''s NumberExpr.Value is a raw string token (''1'', ''1.0'', ''0xFF'', ''1e10''). Plan never says how this is parsed into the IR or how Lua''s number model (one number type in 5.1; int/float in 5.3+) is reconciled with the IR''s int+float split. Result: entity.count == 1 vs entity.count == 1.0 may compile to different IR; bindings of Go int(1) might match one but not the other. Decide: (a) one numeric type (Lua-like), (b) two numeric types (Go-like) with implicit promotion on comparison, or (c) two with no promotion (strict). Spec the lexical-to-typed mapping explicitly. Pin with tests covering ''1'', ''1.0'', ''0xFF'', ''1e10''.'
severity: critical
resolution: 'Plan section ''Numeric type model'' commits to Lua-5.1-style single Number type (float64-backed). All number lexical forms (1, 1.0, 1e10, 0xFF, 1.5e-3) compile to Value{kind: number}. Go int(1) in bindings is converted to float64(1) at binding time. AC10 + TestCompile_NumberLexicalForms pin this.'
status: addressed
---
