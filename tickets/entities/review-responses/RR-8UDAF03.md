---
id: RR-8UDAF03
type: review-response
title: Rename dry run ignores soft-deleted ids that the real rename refuses
finding: IDTakenFolded excluded marked rows while every backend's create and rename refuse a marked id; so a dry run onto a marked id said OK and the rename then conflicted.
severity: significant
resolution: IDTakenFolded now reports live or marked ids on all four backends (pg and sqlite in one SQL probe; mem and fs via markedTaken). The softdelete conformance asserts it and that except skips the marked family.
status: addressed
---
