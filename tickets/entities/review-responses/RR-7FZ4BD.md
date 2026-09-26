---
id: RR-7FZ4BD
type: review-response
title: Resource URI segments not escaped
finding: 'Names like ''a (1).txt'' or containing # or ? produced invalid URIs.'
severity: nit
resolution: Each URI segment is url.PathEscape'd; asserted in TestAttachmentContent.
status: addressed
---
