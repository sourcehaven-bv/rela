---
id: RR-TO2QZ0
type: review-response
title: Message.ReplyTarget is a transient field on the persisted type
finding: 'A `json:"-"` field that only Append fills makes every Get or Messages result carry a misleading zero. Prefer an Append result that carries the reply target, or let Post compute it: a reply''s parent is in the same channel, and the store already refused a bad parent. If the field stays, document Nil/zero: only Append sets it.'
severity: nit
resolution: Kept Message.ReplyTarget with a Zero doc (only Append sets it, never stored); changing Append's return would touch every store and fake for one field.
status: addressed
---
