---
id: RR-FNHC6F2
type: review-response
title: 'allow with only null items accepted (IB review on #2344)'
finding: 'allow: [~], a bare dash or a comment-only item decoded to an empty list and served every tool; develop refused it. Also remote_mcp: with no value counted as absent (#2365).'
severity: significant
resolution: 'refuseEmptyAllow refuses null items; the decoded-value check is back as a backstop; remote_mcp: with no value is refused. Tests cover each form.'
status: addressed
---
