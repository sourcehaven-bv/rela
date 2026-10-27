---
id: RR-QS6DOK
type: review-response
title: Window ordered by marker time hides the channels most likely to be unread
finding: 'ListReads keeps the newest 500 markers by r.at DESC, and unread_channels sorts by the same key. A marker''s at only moves when the reader reads, so the channels with the oldest markers are exactly the ones that collect new messages. A reader with more than 500 markers never sees a busy channel they last opened weeks ago, and among 50+ unread channels the 50 shown are the ones read most recently. Fix: let the store return only candidate channels (`c.last_seq > r.seq` in SQL; the same test in the fs walk) and order the window and the 50-cut by channel activity (rev or last message time, newest first), with key order as the tiebreak. The one-bit window channel stays as documented. Add a testListReads row where the 501st-oldest marker is the only unread channel and must appear.'
severity: significant
resolution: 'Window puts unread candidates first, then orders by new channels.last_at (backfilled), then key; unread_channels uses the same order. testListReads row: oldest marker, only unread, among 501 is returned first.'
status: addressed
---
