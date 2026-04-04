---
id: RR-G01Y
type: review-response
title: Lua return value semantics need clarification
finding: 'Plan says ''Lua returns non-boolean (coerce: nil/false = violation, else = pass)'' but also ''Lua returns nothing (treat as pass)''. These conflict - returning nothing is nil, which would be a violation per the first rule. Clarify: should ''return nothing'' mean ''return true'' (pass) or ''return nil'' (violation)? Recommend: no return value = pass (avoid false positives from forgotten returns).'
severity: minor
resolution: 'Clarified semantics: true=pass, false/nil/no-return=violation, other truthy values=pass'
status: addressed
---
