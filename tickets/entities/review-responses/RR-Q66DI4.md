---
id: RR-Q66DI4
type: review-response
title: Inbox kinds, cursor, limit and the 503 ceiling interact without a defined answer
finding: (1) With default kinds, an inbox_too_large 503 from the waiting half also removes unread, so an agent loses unread exactly when it is busiest. Answer unread on its own, or have the 503 detail suggest kinds=unread. (2) A cursor with kinds=unread answers an empty page; make it 400 invalid_cursor, because the cursor positions waiting entries only. (3) With kinds=unread, entries, count and truncated read as "0 waiting"; omit them or document them. (4) limit does not bound unread_channels (always up to 50); document it. (5) `unread_count` present-when-requested needs a pointer type; a plain int with omitempty drops a real 0. Add these rows to TestInbox_Kinds and TestInboxHandler_Kinds.
severity: minor
resolution: 503 detail names kinds=unread; cursor with kinds=unread is 400 invalid_cursor; waiting fields omitted for kinds=unread; limit not bounding unread documented; unread_count is *int. Rows in both Kinds tests.
status: addressed
---
