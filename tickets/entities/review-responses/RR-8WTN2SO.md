---
id: RR-8WTN2SO
type: review-response
title: Rows means rows returned, not rows scanned
finding: The counter sees rows the store returns. A scan shows only when padding matches what the scan returns. The rule and helper doc did not say so.
severity: significant
resolution: AssertScaleFree doc and collection-reads.md now say to pad with the same type and face and with edges of the relation types read.
status: addressed
---
