---
id: RR-8WTTZJA
type: review-response
title: Relation pulls do not consult the gate
finding: pullRelation and goneRelation build keys from ledger endpoints and report them without checking the puller can read the endpoints.
severity: significant
resolution: 'endpointsReadable checks both endpoints in pullRelation; goneRelation checks them before any report. Tests: relation_endpoint and gone_relation subtests.'
status: addressed
---
