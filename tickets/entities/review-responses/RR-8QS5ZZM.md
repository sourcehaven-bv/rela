---
id: RR-8QS5ZZM
type: review-response
title: '[security] Verify accepts a forged anchor after prefix deletion'
finding: '[security] Deleting the oldest rows and writing a matching audit_anchor row made Verify pass, so prefix deletion was undetectable.'
severity: significant
resolution: Verify now requires the newest audit-prune row to name the anchor seq and hash. A planted prune record cannot be imported. Pinned by ForgedAnchorBreaks.
status: addressed
---
