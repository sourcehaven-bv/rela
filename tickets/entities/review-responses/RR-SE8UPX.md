---
id: RR-SE8UPX
type: review-response
title: refreshView lands stale or over optimistic state
finding: 'Code review: a refresh for entity A resolving after navigation to B replaced B''s view. A refresh inside the checkbox debounce replaced the optimistic content the next toggle is computed from.'
severity: minor
resolution: refreshView keeps a sequence and target check and defers while autosave has pending or in-flight writes. The deferred refresh runs when autosave is idle. EntityDetail.live.test.ts pins both and fails with the guards removed.
status: addressed
---
