---
id: RR-WCMN8A8
type: review-response
title: Own-write check matches old bodies and resends a stale body
finding: ownsContent matches any of the last 64 room bodies, so a history restore to an earlier room body is overwritten. The resend also sends the body as it was when the save went out, which can put an older body back over a peer's newer save.
severity: significant
resolution: 'ownsContent replaced by resendContent(theirs): it resends the room''s current document only when theirs is the room''s latest save. Older room bodies merge. Tests in useBodyCollab and useAutoSave.conflicts.'
status: addressed
---
