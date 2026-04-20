---
id: RR-CL9FG
type: review-response
title: justfile stale 'ratchet baseline' comment on coverage-check recipe
finding: justfile:99 still says `with ratchet baseline` in the recipe comment. Update to `with floor thresholds`.
severity: minor
resolution: justfile:99 updated to `# Check coverage meets floor thresholds (uses go-test-coverage)`.
status: addressed
---
