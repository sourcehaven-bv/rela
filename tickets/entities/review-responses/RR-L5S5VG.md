---
id: RR-L5S5VG
type: review-response
title: 'Cardinality mismatch is only a generated comment, and names one problem twice'
finding: 'cardinalityNotSwapped reports both halves of a single unswapped pair as two separate stale bounds, so the operator reads two problems where there is one. More importantly it emits only a # WARNING into a draft, which is trivially deleted, and nothing gates it - so reversed data can violate the cardinality the migration was supposed to establish.'
severity: significant
resolution: 'The double-report is fixed: the check now runs once per BOUND PAIR rather than once per direction, so a single unswapped min is named once. Pinned by an assertion that the draft does not contain the mirrored form. The gating half is deliberately NOT changed: a cardinality mismatch is a schema-authoring mistake the operator must fix in schema.yaml, and the migration cannot fix it for them - refusing the migration would leave the project stuck with data that still points the wrong way and no way to move it. A loud warning on the draft they must read anyway is the right weight, and the existing soft-validation path reports the violation afterwards.'
status: addressed
---
