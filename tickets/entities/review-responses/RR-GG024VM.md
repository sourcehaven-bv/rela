---
id: RR-GG024VM
type: review-response
title: '[security] ETag view ignored e.Redacted'
finding: '[security] entityView removed only the names from hiddenProperties. If the redacting reader''s Redacted list ever diverged a hidden value would enter the ETag.'
severity: minor
resolution: entityView removes the union of hidden and e.Redacted. TestEntityETag_DropsRedactedNames pins it.
status: addressed
---
