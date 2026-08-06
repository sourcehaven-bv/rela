---
id: RR-QZWI1I
type: review-response
title: Structural lint test banned four names, missing the code this change removed
finding: 'label_derivation_lint_test.go matched four exact declaration strings (''func titleCase('', ''function formatLabel('', ...). Empirically evadable: (a) an arrow function `export const titleCase = (s) => s.replace(/\b\w/g, c => c.toUpperCase())` — the dominant idiom in this frontend — passed; (b) restoring HistoryView.vue''s propertyLabel, one of the derivations THIS change deleted, also passed, because propertyLabel was never in the ban list. A guard that misses the code it was written for is worse than none: it reads like coverage. The name-based approach was the flaw — it banned four names rather than the behaviour.'
severity: significant
resolution: 'Rewrote to match the SHAPE of the transform, not helper names: four regexes covering JS per-word capitalization (replace(/\b\w/), JS first-char (charAt(0).toUpperCase()), Go first-byte (ToUpper(x[:1])), and Go first-rune (runes[0] = unicode.ToUpper). Re-verified all three evasion probes are now caught (arrow function, restored propertyLabel, renamed Go helper ''humanize''), with a clean baseline. The new guard immediately found two live sites the old one missed — see the graph.go finding. Moved from package migration_test to package migration so it reuses the existing findRepoRoot instead of duplicating it, resolving the separate minor finding about repoRoot duplication. Two deliberate allowlist entries with stated reasons: internal/openapi/paths.go (OpenAPI schema names, not labels) and DynamicForm.vue getTemplateLabel (template FILE NAME, no authorable label key; left as follow-up rather than silently widening this diff into unrelated in-flight work).'
status: addressed
---
