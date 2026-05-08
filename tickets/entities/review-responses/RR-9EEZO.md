---
id: RR-9EEZO
type: review-response
title: Unbounded /_search results — palette will OOM the browser on common letters
finding: 'internal/dataentry/api_v1.go:1034-1077 returns every matching entity with no per_page cap. Type a single common letter (''a'', ''e'') in a project with 5000+ entities and you''re rendering 5000 <li> nodes inside a 70vh listbox. Either add a limit query parameter to /_search (default 50) and pass limit=20 from the palette, or slice client-side: results.slice(0, 50). Also: short-circuit on queries shorter than 2 chars (1-letter queries return garbage anyway in a large project).'
severity: critical
resolution: 'Added two named constants: MIN_QUERY_LEN=2 (short-circuits the watcher before any API call) and MAX_RESULTS=50 (slices results client-side). New tests: ''does not call /_search for single-character queries'' and ''caps rendered results at MAX_RESULTS (50)'' (mocks 200 entities, asserts only 50 li nodes render). Backend per_page support is a follow-up; this caps the client side fully.'
status: addressed
---
