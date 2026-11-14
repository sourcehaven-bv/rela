---
id: RR-860EFKX
type: review-response
title: HasPattern still compares exactly
finding: EntityDef.HasPattern compares a pattern with the declared prefixes byte for byte and has no non-test callers.
severity: minor
reason: HasPattern asks whether a pattern string is declared, not whether an id matches, so exact comparison is its meaning. It is unused; removing it is unrelated cleanup.
status: wont-fix
---
