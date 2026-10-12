---
id: RR-AP5G7U
type: review-response
title: UpdateEntity authorized before the read gate
finding: A caller whose only grant is a local role on a hidden row got not-found where a missing id got 403; and a denied-write record was written for hidden ids
severity: significant
resolution: UpdateEntity now validates; reads; gates; then authorizes. Two tests that relied on the old order now give their principals read
status: addressed
---
