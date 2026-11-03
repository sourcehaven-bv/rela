---
id: RR-8NYH33
type: review-response
title: A stranded state reads "working", so the documented agent loop waits forever
finding: 'Approach 3 and AC9 give a stranded session `status: working`. Nothing moves a stranded session until its state is declared again (`realtime/session.go:34-36`). An agent following the tool-description loop sees "working" and waits indefinitely. Change: leave `status` out for a stranded session (`omitempty`, and make it optional in the OpenAPI schema), or document that `stranded: true` overrides it. Name this in the read_session tool description.'
severity: minor
resolution: 'A stranded session reads status failed with stranded: true, documented and named in the read_session loop (AC 12, AC 25).'
status: addressed
---
