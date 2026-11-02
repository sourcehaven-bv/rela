---
id: RR-47LZSP
type: review-response
title: Fixture guard is weaker than the dataentry one
finding: 'internal/mcp/tools_channels_test.go:895-898 and 925-927: defaultInYAML nests two strings.Replace calls and the guard checks strings.Contains on the results. internal/dataentry/channels_schema_test.go:236-250 checks strings.Count(src, old) == 1 per edit, which also catches an old string that matches twice. Use the same per-edit count check (a helper func instead of a package var) so the two fixtures follow one pattern.'
severity: nit
resolution: "addressed\tdefaultInYAML is now a helper func that checks strings.Count == 1 per edit, the same pattern as defaultInRealtime in dataentry."
status: addressed
---
