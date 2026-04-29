---
id: RR-TEGZP
type: review-response
title: Cache namespace collision risk between validation and real scripts
finding: 'Validation sets runtime.SetScriptPath(''validations/'' + ruleName). The shared *lua.Cache is keyed on this path. If a real script lives at validations/foo.lua and an inline rule is named foo.lua, their cache namespaces collide silently. Inline rules use the rule name verbatim with no extension — collision space is rule names that look like file paths. Location: internal/validation/lua.go:101.'
severity: minor
resolution: Inline validation rules now use envelope path 'validation:<rule-name>' (colon, no slash) so their chunkname and rela.cache.* namespace cannot collide with a real script at validations/<rule-name>.lua. File-backed rules still use 'validations/<file>'. New test TestLuaValidation_InlineRuleUsesDistinctCacheNamespace asserts the path and frame alignment. Commit d3eda91.
status: addressed
---
