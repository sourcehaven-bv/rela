---
id: RR-0UPVRQR
type: review-response
title: Body is read before admission
finding: Up to 512 KiB is read and parsed before the token bucket.
severity: minor
resolution: 'Intended, as on _batch: a comment in the handler says so; both steps are bounded and cheap.'
status: addressed
---
