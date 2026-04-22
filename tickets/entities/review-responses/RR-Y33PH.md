---
id: RR-Y33PH
type: review-response
title: eslint relaxation scope is a trap for future helper files
finding: Rule relaxed for pages/ and tests/fixtures.ts. A new tests/helpers.ts would get the spec-ban applied. Move helpers to e2e/helpers/ or e2e/support/ and document it.
severity: significant
reason: No helper files exist yet. Will address when the first one is added — a sibling directory `e2e/support/` can then be allowlisted in eslint.config.js. Documenting in AGENTS.md per suggestion.
status: deferred
---
