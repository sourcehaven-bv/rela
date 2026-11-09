---
id: RR-RSMNFI0
type: review-response
title: Exit codes collide
finding: --exit-code uses 1 for differences and errors also exit 1. Use 2 for errors.
severity: minor
resolution: '--exit-code: 1 for differences and 2 for errors (printed to stderr). Without it errors exit 1 and differences 0.'
status: addressed
---
