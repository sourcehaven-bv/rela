---
id: RR-MAILC1
type: review-response
title: Invalid recipient addresses would consume bounded retries instead of skipping
severity: significant
finding: The first implementation returned errors for missing, non-scalar, blank, or malformed addresses. The queue would retry unchanged graph data, contradicting the ticket's safe-skip behavior and wasting the recipient's bounded retry budget.
status: addressed
resolution: Runtime address failures now log only the recipient ID and property, then finish that recipient child successfully. Valid peers remain independent and malformed addresses are never sent or logged.
---

The first implementation returned errors for missing, non-scalar, blank, or
malformed addresses. The queue would retry unchanged graph data, contradicting
the ticket's safe-skip behavior and wasting the recipient's bounded retry budget.
