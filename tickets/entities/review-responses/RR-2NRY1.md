---
id: RR-2NRY1
type: review-response
title: 'JSON encode: Lua table array vs object ambiguity'
finding: 'Lua tables don''t distinguish arrays from objects. json_encode({"a", "b"}) should produce ["a","b"] but json_encode({x=1}) should produce {"x":1}. The plan mentions ''recursive Lua table to Go value conversion'' but doesn''t specify the heuristic. Standard approach: if all keys are consecutive integers starting at 1, treat as array; otherwise treat as object. Mixed tables (both integer and string keys) need a documented behavior — either error or pick one. This edge case has bitten many Lua-JSON libraries.'
severity: significant
resolution: 'Documented array/object heuristic: consecutive int keys = array, any string keys = object'
status: addressed
---
