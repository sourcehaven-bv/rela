---
id: RR-CNTE7L
type: review-response
title: No guard against out-of-order next-action responses
finding: load() has no abort or sequence; a slow response for page A can render on page B, including another entity's context hint.
severity: significant
resolution: 'Planned: abort the previous request via AbortController and drop stale responses by sequence; hide the card when the placement tuple changes. Deferred-promise test.'
status: addressed
---
