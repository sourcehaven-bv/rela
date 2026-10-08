---
id: RR-H4PNUY
type: review-response
title: Phase 3 places the anchor after the quoted middle block is rewritten or deleted
finding: The middle was weighted by length, so a short middle was outvoted by long intact endpoints; a deleted middle cost only 0.15. A comment on 'Do not reboot.' landed on 'Always reboot first.'
severity: critical
resolution: A quote with a middle now needs the middle present (j > i+1) and similar on its own (>= 0.6). Multi-paragraph quotes skip phase 2, which matched the first paragraph alone. Pinned by TestResolveCrossBlockNeverMisplaces (short middle rewritten, middle deleted).
status: addressed
---
