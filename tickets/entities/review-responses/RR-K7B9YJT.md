---
id: RR-K7B9YJT
type: review-response
title: Refusal tests loop without t.Run
finding: ai and mail refusal tests looped over names without subtests.
severity: minor
resolution: Wrapped each case in t.Run.
status: addressed
---
