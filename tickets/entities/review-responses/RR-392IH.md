---
id: RR-392IH
type: review-response
title: allowedOptionList has dead code and copy-paste variable name
finding: internal/lua/cache.go allowedOptionList declares `out := ""`, writes into a uniquely-named variable suffixed with a line number (copy-paste artefact from a rewrite), then concatenates. Simplify to a single strings.Builder or use sort.Strings.
severity: significant
resolution: Replaced dead-code allowedOptionList body with sort.Strings(keys) + strings.Join(keys, ', '). Added sort to imports.
status: addressed
---
