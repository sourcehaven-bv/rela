---
id: RR-IJG9
type: review-response
title: DarkMode type needs UnmarshalJSON/MarshalJSON for API compatibility
finding: 'DarkMode has custom UnmarshalYAML for the three-way union (auto/false/object) but no JSON equivalent. The palette API handler uses json.Decode which will fail to deserialize {"dark": {"accent": "#818cf8"}} into the Mode/Explicit struct. Also MarshalJSON is needed so GET returns the correct format. This is a backend fix required before the frontend can send dark mode overrides.'
severity: critical
resolution: Add UnmarshalJSON/MarshalJSON to DarkMode in palette.go following the same pattern as the YAML methods
status: addressed
---
