---
id: RR-7Q3DYT
type: review-response
title: The no_unread request row pins nothing about the flag
finding: 'internal/realtime/unread_test.go:44 posts props {"no_unread": true} and expects 400. Every undeclared property is refused that way, so the row would pass even if the service later read a client value into NoUnread from somewhere else, such as a PostInput field. The flag cannot come from a request because PostInput has no such field and the service sets NoUnread only from mt.NoUnread (service.go:452). Delete the row, or pin the real property: post a counting type and assert lastAppend.NoUnread is false whatever the request carries (already done in the loop above).'
severity: minor
resolution: ADDRESSED deleted the no_unread request row and its unused import; the godoc says the flag comes only from the type.
status: addressed
---
