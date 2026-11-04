---
id: RR-QN7IOTA
type: review-response
title: Type 422 now precedes the manager ACL 403 on PATCH bodies
finding: TestPatchRelations_DisallowedTypeWithDeniedRemoveWritesNothing expects 403 for a disallowed re-point whose remove is denied. Phase A now refuses with 422 first. Check that answering 422 before 403 leaks nothing and keep the denied-remove coverage with an allowed target.
severity: minor
resolution: The 422 depends only on the path type (caller-known) and readable peer types and the schema (config is not secret). The test keeps its intent with an allowed category target.
status: addressed
---
