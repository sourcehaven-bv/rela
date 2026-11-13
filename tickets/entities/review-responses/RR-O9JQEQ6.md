---
id: RR-O9JQEQ6
type: review-response
title: Plan misstates join evaluation and lacks a lost-lock test
finding: 'The plan said a join when: is evaluated with the visible: check; it is evaluated over the raw subject. The edge case of a lost postgres lock had no test.'
severity: minor
resolution: The plan now says the join is evaluated over the raw subject as system routing and its verdict is never shown. TestFlowWorker_StopsLeadingWhenTheLockIsLost covers a failing check.
status: addressed
---
