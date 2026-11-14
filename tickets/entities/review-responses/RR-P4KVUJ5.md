---
id: RR-P4KVUJ5
type: review-response
title: If-Match relation delete runs the rule check unguarded
finding: checkRelationRules ran on every If-Match relation delete, including unruled ones.
severity: nit
resolution: Guarded with relationWritesChecked.
status: addressed
---
