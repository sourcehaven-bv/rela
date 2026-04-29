---
id: RR-DSIWT
type: review-response
title: Dead ResolvedField struct kept alive only by deleted templates
finding: internal/dataentry/helpers.go:158-176 defines a ResolvedField struct whose own doc comment says "Used by form templates to render property inputs consistently." The form templates are now gone. Repo-wide grep for ResolvedField returns only the type definition site — no constructor, producer, consumer, or test. Leaving an exported struct named ResolvedField pointing at deleted templates is a landmine for the next reader who greps for "form field" types.
severity: critical
resolution: Deleted the ResolvedField struct from internal/dataentry/helpers.go (was lines 158-176). Verified by `grep -rn ResolvedField --include='*.go'` returning empty. `just build` and `just test` both still green afterward, confirming nothing referenced the type.
status: addressed
---
