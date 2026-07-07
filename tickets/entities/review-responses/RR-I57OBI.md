---
id: RR-I57OBI
type: review-response
title: Loader has no warn channel; labels validation must be silent or a real diagnostic
finding: 'The plan says ''optional load-warn if a labels key isn''t in values'', but the loader has no warn severity — validate() accumulates hard errors into SchemaValidationError (loader.go:227-243). Options: (a) silent tolerance (matches permissive-storage philosophy, simplest, recommended), or (b) a real hard-error diagnostic added alongside the enum-empty check in validatePropertyDefs (loader.go:685) AND validateCustomTypes (loader.go:442). Pick one; the plan currently assumes a channel that doesn''t exist. A labels key not in values is harmless (never rendered), so silent tolerance is defensible.'
severity: significant
resolution: Chose silent tolerance (option a). A labels key not in values is harmless (never rendered) and matches the permissive-storage philosophy; no loader diagnostic added. Verified by TestParse_EnumLabels (labels parse without error).
status: addressed
---
