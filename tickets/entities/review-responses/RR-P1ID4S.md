---
id: RR-P1ID4S
type: review-response
title: Conformance suite missing content-only and false-drop cases
finding: RunVisibleFieldSearchTests covered happy paths + hidden-func-error but not a content-match-survives case nor a caller-false-drop case.
severity: minor
resolution: Added ContentMatchSurvivesHiddenProperty and CallerCannotFalseDropViaIDOrContent. The stale-entity found=false path is exercised by the fail-closed code structure + the dataentry-level flow; it can't be desynced through the public store/observer API at the conformance layer (documented).
status: addressed
---
