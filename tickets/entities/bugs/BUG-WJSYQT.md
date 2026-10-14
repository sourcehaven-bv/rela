---
id: BUG-WJSYQT
type: bug
title: List filter and sort evaluate hidden fields (value disclosure)
description: List filter[] and sort evaluate fields the principal cannot see, so a hidden value can be recovered by search.
priority: high
why1: applyV1Filters, applyV1Sorting and planListPushdown read or push any property the caller names. None of them asks whether the principal may read that property.
why2: Field redaction (visible:) is applied only when a row is serialized. Filter membership and sort position are computed from the raw row before that point.
why3: 'The list filter and sort predate field-level visible:. When visible: was added and the search oracle was closed (TKT-GGQ0JT), the list pipeline was not revisited.'
why4: Redaction is treated as a property of the response body, not of the data a request may compute over, so each derived computation (search, ETag, roll-up, filter, sort) must remember it.
why5: No shared seam forces a derived computation through the field-visibility decision, and no test enumerates the computations that read raw values.
prevention: 'gatedListQuery is the only query the list filter, sort, relation pass and store pushdown accept, and only gateListQuery builds it (TestGatedListQuery_BuiltOnlyByTheGate). One key parser serves the gate and every list consumer. List q= searches through the same field-provenance filter as /_search, and a relation filter compares the redacted neighbour title. internal/dataentry/list_field_gate_test.go pins every operator, sort, the when: case, the shared list pipeline, q= and relation filters (MEAS-LIST-FILTER-SORT-GATED); a differential test keeps the rule in step with FieldVerdicts; the golden corpus records the 400s. This covers the list endpoint and the paths built on its pipeline (export, _position for a list scope, sidebar counts); /_search structured terms and sort: are not covered and are tracked in BUG-BINAR2.'
started: "2026-10-13"
completed: "2026-10-14"
status: done
---

## Description

List `filter[]` and `sort` evaluate properties the principal cannot see.
`applyV1Filters` (internal/dataentry/api_v1.go:2176), the sort (:2344, :2378)
and the pushdown in listpushdown.go never check field visibility.

Reproduced by the TKT-0M4H0S security review on the golden corpus fixture
(memstore), as the viewer, whose `secret` and `note` are hidden by `visible:`:
- `?filter[secret]=bravo-secret` returns TKT-2 (total 1); `=nope` returns 0.
- `?filter[secret][contains]=brav` matches, `bx` does not; `[gt]`/`[lt]` work, so a binary search recovers the full value.
- `?sort=secret` orders by the hidden raw value.

This is the value disclosure RR-5KEF8E rejected for gantt and BUG-IPO4UH closed
for search. pgstore pushes the same query (not yet verified there).

## Fix

Reject `filter[]` and `sort` on a property the principal cannot see
unconditionally (400). For a `when:`-conditional field, reject or evaluate on
redacted rows after the gate, never in the pushdown. Pin with corpus cases and a
unit test per operator.
