---
id: RR-GVXUIQ
type: review-response
title: Property/type names are not charset-validated at metamodel load — DDL injection surface
finding: Property/type names aren't charset-validated at metamodel load (validatePropertyDefs only checks reserved names + types). Names are interpolated into DDL string literals (properties->>'p', WHERE type='t') which can't be bind-parameterized and have no quote_literal helper. Genuine DDL injection from on-disk config. Add a strict charset validator + reconciler refusal + literal escaping.
severity: critical
status: open
---

The plan assumes type/property names are safe because they're "known" operator
config. But `validatePropertyDefs` (metamodel/loader.go:654) only checks
reserved names + type correctness — there is NO regex constraining a property
name to [A-Za-z0-9_]+ (the only identifier regex, validIDPrefixBase, is for
entity-ID prefixes). So a property named `email' OR '1'='1` passes metamodel
load today. Compounding: the DDL interpolates the name/type into STRING LITERALS
(`properties->>'<prop>'`, `WHERE type='<type>'`), not identifiers —
`pgQuoteIdentifier` (listener.go:379) doesn't apply, there's no quote_literal in
the package, and CREATE INDEX can't take bind parameters. Genuine DDL-injection
driven by on-disk config.

REQUIRED (at least two): (1) add a strict property/type-name charset validator
at metamodel load, and have the reconciler REFUSE (unenforce+warn) any name
outside it rather than trusting load-time validation; (2) use a proper
string-literal escaper (single-quote doubling; dollar-quoting is itself
attackable if the tag can appear in the value); (3) allowlist-charset inputs
first. "Known names" ≠ "safe names" until something enforces the charset — today
nothing does.
