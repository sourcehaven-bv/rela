---
id: RR-F52QL
type: review-response
title: stubEntityManager copy-pasted in two test files
finding: internal/dataentry/document_script_test.go:28-54 (docTestStubEM) and internal/script/executor_test.go:23-51 (stubEntityManager) are literal duplicates. If entitymanager.EntityManager grows a method both must be updated. Should live in a shared test helper package.
severity: minor
reason: 'Deferred: low urgency — EntityManager interface is stable, stub duplication is small. Filed as TKT-GOLNP (extract to shared entitymanagertest package). Will address when either EntityManager grows or we need a recorder-style fake.'
status: deferred
---

From post-impl cranky review.

Deferred: extract to internal/entitymanager/entitymanagertest package (or
equivalent). Low urgency — the stubs are small and the EntityManager interface
hasn't changed frequently. Filed as backlog ticket.
