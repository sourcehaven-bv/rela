---
id: RR-1RUR2G
type: review-response
title: No test exercised real mermaid output
finding: Every mermaid test mocks render() so nothing checked that what mermaid actually emits survives sanitization.
severity: significant
resolution: 'Real mermaid cannot run under jsdom (getBBox) so added e2e/tests/mermaid.spec.ts: real browser and real mermaid; asserts label markup survives and hostile-label img / handlers are stripped. Passes.'
status: addressed
---
