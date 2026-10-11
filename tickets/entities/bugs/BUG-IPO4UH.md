---
id: BUG-IPO4UH
type: bug
title: Search returns hits whose only match is a visible:-hidden field on the bleve backend
description: 'On the default fs build the field-visibility search filter (TKT-GGQ0JT) asks bleveindex.Index.MatchedFields which fields a hit matched. That matcher over-reported: it unioned a substring pass (search.MatchTextFields) and the query''s analyzer tokens into its answer. Query ''foo,bar'' matches only a hidden property ''foobar'' in bleve (one edit from the raw word), but the matcher also reported the visible title ''foo visible'' (analyzer token ''foo''). SearchVisibleFields then kept the hit. A reader without access to the property can confirm its value by searching: a hidden-field match oracle. pgstore computes provenance on its own path and is out of scope here.'
priority: high
effort: s
why1: SearchVisibleFields kept the hit because bleveindex.MatchedFields reported the visible title as matched, though bleve matched only the hidden code property.
why2: 'MatchedFields unioned a substring pass and the query''s analyzer tokens into its answer. Both match more than bleve does: the raw word ''foo,bar'' is what bleve fuzzy-matches, while its analyzer tokens ''foo'' and ''bar'' hit the title.'
why3: 'The FieldMatcher contract asked for a superset: it treated an extra reported field as safe and only a missing one as dangerous. For a filter that keeps a hit when any visible field matched, the opposite holds: an extra field keeps a hit that should be dropped.'
why4: The contract was written from the availability side (avoid false drops) without checking which error direction widens visibility for this filter. No test compared the matcher's answer with what the index actually returned.
why5: Security filters that re-derive a backend decision outside the backend have no oracle test against the backend itself, so a disagreement between the two goes unnoticed.
prevention: 'The FieldMatcher godoc now states the safe direction: under-report is allowed, over-report never. TestMatchedFields_AgreesWithTheIndex uses the bleve index as the oracle and fails if MatchedFields reports a field the index did not match on. Any future re-derivation of a backend''s match decision for an ACL filter should get the same oracle test.'
started: "2026-10-11"
completed: "2026-10-11"
status: done
---

## Reproduction

`TestSearchVisible_BleveHiddenOnlyMatchDropped`: an entity with visible title
"foo visible" and `visible:`-hidden property code "foobar". Searching "foo,bar"
returns the entity through `SearchVisibleFields`, though bleve matched only the
hidden code.

## Fix

`bleveindex.Index.MatchedFields` now applies bleve's own rules per field: the
raw lowercased query word against the field's analyzer tokens (fuzzy within one
rune edit, or a whole-token wildcard), plus the id exact/prefix and id-token
rules. The substring union is gone. The `search.FieldMatcher` contract now says
a matcher may under-report (the hit is dropped) but must never over-report.

Tests: `TestMatchedFields_AgreesWithTheIndex` (the index is the oracle),
`TestMatchedFields_IDTokens`, `TestSearchVisible_BleveHiddenOnlyMatchDropped`.
