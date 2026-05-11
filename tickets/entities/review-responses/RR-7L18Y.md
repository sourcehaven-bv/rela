---
id: RR-7L18Y
type: review-response
title: Lua multi-return is string.gsub semantics, not io.open — document explicitly
finding: 'Plan claims io.open is precedent. But io.open returns (value, err) where value is nil iff err is set — mutually exclusive. This ticket has (entity, warnings) where both can be non-nil. That''s string.gsub semantics. Confusing the two will bite: user familiar with io.open writes ''if w then return error(w) end'' and now soft conditions explode the script — silently regressing AC18''s promise. Hard errors still come through RaiseError so pcall changes only for soft. Recommendation: document explicitly in Lua scripting docs and at luaUpdateEntity definition: ''returns (entity, warnings) like string.gsub, not (entity, error) like io.open. Hard failures still raise. Warnings are advisory and nil when none.'' Add AC: second return is nil (not empty string) when no warnings. From design-review F8.'
severity: significant
resolution: 'Lua multi-return documented explicitly as ''string.gsub-style, not io.open-style'' in: Scope section, Research section, Layer 3 spec, Risk #5, AC25 (asserts nil not empty when no warnings, defends against io.open-style misinterpretation), AC32 (doc requirement). WarningsToTable helper returns LNil when len==0 to make the contract enforceable. Hard errors still raise via RaiseError (AC26).'
status: addressed
---
