---
id: BUG-F1LTP1
type: bug
title: Filter pipeline silently degrades unknown operators at every layer (SPA rewrites to eq; API drops the clause and returns the superset)
description: |-
    Two independent silent fallbacks compounded: (1) the SPA's toApiOperator() mapped ANY unknown UI operator to `eq` (`OPERATOR_MAP[op] || 'eq'`), so a config operator the SPA didn't know was silently rewritten to equality — and OPERATOR_MAP was ALSO missing the documented `in` operator, so even valid `in` configs degraded to eq. (2) The API's applyV1Filters handled unknown wire operators and malformed keys with slog.Warn + continue, dropping the clause and returning the UNFILTERED superset — while code comments and TestV1FilterUnknownOperator described this as "fail-closed". The relation-filter path had a third variant (drop ALL rows, RR-6RF60V). Verified live: filter[status][matches]=x returned all 192 tickets. Net effect: a broken operator produced wrong data with zero user-visible signal at any layer.
priority: high
effort: m
why1: A filter with an unevaluable operator produced wrong results (zero rows via the SPA's eq rewrite, or the full superset via the API's clause drop) with no error anywhere a user or config author would see it.
why2: Each layer independently chose a silent fallback — SPA `|| 'eq'`, API warn+continue — and each looked reasonable in isolation ("be robust to bad input").
why3: The API's skip-the-clause behavior was actually believed to be fail-closed (comments and a test asserted it), but dropping a filter clause is fail-open at the result level — the misconception was pinned by a test instead of caught by one.
why4: '"Robustness" was implemented as silent degradation instead of visible rejection, and no end-to-end test sent an invalid operator through config → SPA → API to observe what a user actually gets.'
why5: 'Systemic: DEC-HWZHA already classifies malformed wire format as hard-400 territory, but the filter params predated that discipline and were never audited against it; server logs were treated as a substitute for client-visible errors.'
prevention: 'Unknown operators are now loud at both layers: toApiOperator passes unknown operators through unchanged (console.warn) so the server names them, and applyV1Filters/applyRelationFilters return errBadFilter → HTTP 400 invalid_filter (per DEC-HWZHA). Pinned by TestV1FilterUnknownOperator (incl. the `=~` case), TestV1FilterMalformedKeyRejected, the relation-filter 400 cases, and the filters.ts pass-through test. OPERATOR_MAP gained the missing `in` entry with a pinning test.'
status: done
---

Fixed on branch `bug/filter-pipeline-and-empty-chips`.

- `frontend/src/utils/filters.ts`: OPERATOR_MAP += `in: 'in'`; toApiOperator
  warns + passes unknown operators through instead of rewriting to `eq`.
- `internal/dataentry/api_v1.go`: errBadFilter sentinel; applyV1Filters and
  applyRelationFilters return it for malformed keys / unknown operators;
  writeListPipelineError maps it to 400 `invalid_filter` echoing the bad
  key/operator (client-caused, no internals). Supersedes the RR-6RF60V
  zero-rows drop on the relation path — both fail closed, the 400 also
  names the problem.
- `docs/data-entry.md`: unknown-operator paragraph rewritten to match.
