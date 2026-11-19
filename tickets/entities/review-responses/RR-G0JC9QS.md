---
id: RR-G0JC9QS
type: review-response
title: '[security] Settle mixes identities on the creator''s ctx'
finding: '[security] settle stamped system:flow on the creator''s ctx, which still held the creator''s Request, read world and read time.'
severity: minor
resolution: settleStarted runs on a fresh ctx with only system:flow stamped; every write rebinds as before.
status: addressed
---
