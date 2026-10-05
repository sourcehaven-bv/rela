---
id: RR-U5ZJDR
type: review-response
title: '[security] Zero Attachments value fails every write'
finding: AttachmentsOf returns a zero value for a manager without file properties, and it fails every write.
severity: nit
reason: 'Failing closed is the intended behaviour: a metamodel without file properties has no attachment writes to allow.'
status: wont-fix
---
