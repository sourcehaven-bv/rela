---
id: RR-LRLSJ
type: review-response
title: luaCacheMemoize validates after converting first values
finding: 'In luaCacheMemoize, after PCall succeeds, the loop walks returns with validateRepresentable and on the first bad return calls RaiseError. The stack isn''t corrupted (caller''s PCall frame handles it) but the design is fragile: the loop converts values before it has finished validating all of them. Two-pass (validate all first, then convert) would be clearer and rule out hypothetical side-effect interleaving.'
severity: significant
resolution: 'Split luaCacheMemoize''s post-PCall loop into two passes: first validate all returns via validateRepresentable, then convert via luaValueToGo. Makes ordering explicit and rules out hypothetical side-effect interleaving.'
status: addressed
---
