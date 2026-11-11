---
id: RR-HAGWCT2
type: review-response
title: No channel key field; waiting identity misses re-entry
finding: InboxChannel has no key; keying waiting by state hides a session that re-enters the same state.
severity: significant
resolution: Identity m:<rowKey>#<seq> and w:<rowKey>@<session.generation>.
status: addressed
---
