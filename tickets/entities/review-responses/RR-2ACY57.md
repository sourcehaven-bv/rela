---
id: RR-2ACY57
type: review-response
title: Timeout maximum exceeds common proxy timeouts
finding: 'Architecture design review: a 60 s watch plus a pass exceeds nginx''s default 60 s read timeout.'
severity: minor
resolution: Maximum is 50 s, default 30 s.
status: addressed
---
