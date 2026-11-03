---
id: RR-XNKF89
type: review-response
title: Timing oracle from post-filtering
finding: Filtering in Go makes latency grow with hidden mentioners.
severity: minor
resolution: Gating happens in Go after the index read, so latency still grows with hidden mentioners, bounded by the 10,000-row scan budget per call. The cursor can only name a served id, so a caller cannot move the window through hidden rows to probe them. The remaining coarse signal is documented as accepted in docs/acl-security.md.
status: addressed
---
