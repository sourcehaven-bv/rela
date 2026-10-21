---
id: TKT-1TZ6ZO
type: ticket
title: Check whether list ?q= matches on visible:-redacted field values
kind: chore
priority: high
effort: s
status: wont-fix
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

## Resolution

Already fixed by #1907 (acd88b0c4, the ACL boundary refactor). freeTextIDs now
calls `visibility.SearchScoped`, which adds the caller's field scopes to the
query and runs it through `search.VisibleSearcher.SearchVisible`. Row and field
scopes apply inside the backend query, so redacted values do not match and the
limit counts only rows the caller may read. Closed without a separate change.
