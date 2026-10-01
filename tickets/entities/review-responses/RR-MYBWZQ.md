---
id: RR-MYBWZQ
type: review-response
title: Truncation dropped combination findings first
finding: Exposures cut at 200 in output order, so the C2/C3 combination findings (hardest to spot by hand) were the ones dropped.
severity: significant
resolution: The cut keeps C3, then C2, then C1; output order stays C1, C2, C3. Covered by the Truncated test.
status: addressed
---
