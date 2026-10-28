---
id: RR-76PDSQ
type: review-response
title: A crashed weekly run closed the tracking issue
finding: archplan exits 1 on an unexpected crash as well as on findings. The step accepted rc 1 with an empty archplan.json and the issue step read the empty count as zero and closed the issue.
severity: critical
resolution: The scan step now requires well-formed JSON with findings and ratchet rows before it trusts rc 1, and the report job checks that both counts are numbers.
status: addressed
---
