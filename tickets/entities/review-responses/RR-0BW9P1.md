---
id: RR-0BW9P1
type: review-response
title: 'Large channels: memory, render cost, scroll'
finding: '§3 holds every loaded message forever, and §0 renders markdown on each render. A long session on a busy channel grows without bound, and marked plus DOMPurify per message per re-render is the visible cost. Fix: memoize rendered HTML by `seq:rev`; cap the held list (drop the oldest beyond N and set hasOlder again); keep the scroll position on prepend, and auto-scroll on new messages only when already at the bottom. Add a story with 500 messages.'
severity: minor
resolution: Rendered HTML is memoized by seq and rev, the held list is capped at 500, scroll position is kept on prepend, and a 500-message story is added (R19).
status: addressed
---
