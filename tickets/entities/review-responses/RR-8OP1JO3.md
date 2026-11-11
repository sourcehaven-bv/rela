---
id: RR-8OP1JO3
type: review-response
title: Inverse SPA guard passes on any script failure
finding: release.yml treated every non-zero exit of check-embedded-spa.sh binary as 'SPA absent' with stderr discarded, so a scan error or missing index.html passed the guard. The editor bundle was not checked.
severity: significant
resolution: Added an 'absent' mode to scripts/check-embedded-spa.sh that fails on scan errors, needs a non-empty asset list, and also checks the editor bundle marker. release.yml calls it with stderr visible.
status: addressed
---
