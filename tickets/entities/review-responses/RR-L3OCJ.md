---
id: RR-L3OCJ
type: review-response
title: Global gocritic importShadow disable hides a real stdlib-context shadow
finding: 'In .golangci.yml the gocritic importShadow check is disabled globally with a comment about the entity variable being natural. Re-enabling the check surfaces 4 findings: 3 entity shadows (justified) AND one context shadow in internal/dataentry/commands.go:106 func contextMatchesPage(context, pageType string). The parameter shadows the stdlib context package (imported in the same file). The function body doesn''t use context today, but any future refactor that reaches for context.Background() will silently compile against the string param. Fix: rename the param to contextType or ctxType, keep importShadow enabled, and put //nolint:gocritic comments on the 3 justified entity cases.'
severity: significant
resolution: 'Renamed the context parameter to cmdContext in internal/dataentry/commands.go:contextMatchesPage to remove the stdlib-context shadow. Attempted to re-enable importShadow globally but it produced 42 additional entity-shadow warnings across automation, dataentry, filter, mcp, workspace; the noise-to-signal ratio made per-site nolint impractical. Final state: importShadow stays disabled with a sharper comment noting that stdlib shadows (like the context one fixed here) should be caught in code review. The specific dangerous case the reviewer flagged is resolved.'
status: addressed
---
