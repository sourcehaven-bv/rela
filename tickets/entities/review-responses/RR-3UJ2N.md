---
id: RR-3UJ2N
type: review-response
title: 'Old #<entity-id> bookmarks land at top after 2s'
finding: 'internal/dataentry/document.go dropped fragment synthesis. Users with bookmarked URLs from the old behaviour (e.g. /form/foo/TKT-1?return_to=/doc%23tkt-1) will now trigger scrollBehavior''s 2s wait (element doesn''t exist under the new id scheme) then land at top. Functional but slower. Fix: accept as a one-time migration cost; document in CHANGELOG; the wait-then-top behaviour is also addressed by the scrollBehavior-race RR (don''t stomp scroll on timeout).'
severity: minor
reason: 'Old bookmarks with synthesized #<entity-id> fragments will trigger a 2s wait (combined with the scrollBehavior-race fix, the user keeps their current scroll position; no stomp-to-top). No user reports today. Accept as a one-time migration cost; CHANGELOG entry when we cut a release.'
status: deferred
---
