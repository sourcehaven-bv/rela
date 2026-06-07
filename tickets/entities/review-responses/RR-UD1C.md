---
id: RR-UD1C
type: review-response
title: '"Behaviour-equivalent" is impossible without enumerating the diff explicitly'
finding: |
  Three concrete behaviour shifts are baked into the proposal but never listed: (a) cards date fields go from raw ISO to locale-formatted; (b) cards rrule fields go from raw string to .toText(); (c) PropertyDisplay's shouldUseBadge requires propType OR isEnumProperty(prop) -- but the widget path will badge whenever propertyDef.values?.length > 0, which is a slightly different predicate. AC #7 promises "structural equivalence" but the matrix test will fail on any of these unless equivalence is defined up front.
severity: critical
resolution: |
  Plan revised. Added a "Known behaviour deltas" table to the ticket listing 7 explicit diffs (date formatting in cards/list, rrule formatting in cards/list, inaccessible lock now showing in cards/list, badge predicate change). The pre/post DOM diff at merge confirms the diff matches this table and nothing else. Any unlisted diff is treated as a regression and must be fixed before merge.
status: addressed
---
