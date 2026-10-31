---
id: RR-QU963E
type: review-response
title: validator.persist also decides NoUnread
finding: 'internal/realtimecfg/validate.go:434: the function extracted for cognitive complexity is named persist but also compiles `unread`. The godoc says so, but a reader looking for where NoUnread is set will not look in persist. Consider naming it persistKeys, or move the two NoUnread lines back into messageType after the persist call: they only need mt.Persist.'
severity: nit
resolution: 'ADDRESSED NoUnread is no longer set in persist: a free function unreadOf, called from messageType after persist, checks and compiles unread, including the signal refusal; persist''s godoc no longer mentions it.'
status: addressed
---
