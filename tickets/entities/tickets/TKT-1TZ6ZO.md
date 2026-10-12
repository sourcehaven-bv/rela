---
id: TKT-1TZ6ZO
type: ticket
title: Check whether list ?q= matches on visible:-redacted field values
kind: bug
priority: high
effort: s
status: backlog
---

## Description

GET /api/v1/{plural}?q= resolves free text through freeTextIDs
(internal/dataentry/helpers.go), which queries the raw svc.Searcher rather than
the visibility-gated searcher, then intersects ids with ACL-scoped rows. Row
gating holds, but if the raw index stores values of fields redacted by visible:,
a visible entity matches on a value the caller cannot read: a field-value
oracle. Hidden entities also count toward the 1000-hit cap. Found by the
security review of the live-views commit (RR on TKT-HJ5Z9Y). Verify per backend;
if confirmed, route through searchVisibleHits / FieldVisibleSearcher and cap
after filtering.
