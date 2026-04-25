---
id: RR-1O10G
type: review-response
title: Error message drops the 'why' the docstring spells out
finding: The ban's source comment carefully explains the typed-helper alternatives and Go integration test rationale, but the eslint `message:` itself is a single short sentence ('add a Go integration test instead'). The neighboring `request.fetch` rule has a two-sentence message that names the helper and the carve-out. Eslint output is what the developer sees at 4 PM on Friday; make it carry the explanation.
severity: minor
resolution: 'Rewrote the message to two sentences matching the `request.fetch` precedent: explains *what* the rule means (HTTP-shape testing belongs in Go integration tests under internal/dataentry/) and *what to do instead* (use the typed api helpers — createEntity, getEntity, listRelations, updateEntity — for seed and verify in UI tests). Both the dotted and bracket variants share the same explanation, differing only in whether they say `api.rawRequest` or `api[''rawRequest'']`.'
status: addressed
---
