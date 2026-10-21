---
id: RR-MQXYX3
type: review-response
title: Conformance harness writes many types that fsstore's test schemas do not declare
finding: 'storetest writes page, task, doc, node, user, policy, item, note, issue, concept, atype, t, req and others. defaultTestSchemas (fsstore/persistence_test.go) declares only requirement, solution, artifact, document, thing, ticket, decision and feature. storetest/differential_test.go declares only ''a'' and ''b'' but writes ticket and feature. TestConformance, the fuzz targets, RunTxStressTest and FuzzDifferential will fail broadly. Approach step 4 (''search for others'') understates this. Enumerate the full set in the plan. Better: export the harness''s type list from storetest (for example storetest.EntityTypes) and build fsstore''s test schemas from it, so a new harness test cannot drift. Fix the differential schemas too.'
severity: significant
resolution: 'Plan updated: storetest exports its type list and fsstore''s test schemas are built from it, so harness tests cannot drift (AC9).'
status: addressed
---
