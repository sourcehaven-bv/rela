---
id: RR-R8W1A
type: review-response
title: 'Prototype config combines id_type: sequential with id_prefixes [DEC-, ADR-]'
finding: 'prototypes/data-entry/project/metamodel.yaml:62-69 has decision with id_type: sequential and id_prefixes: [DEC-, ADR-]. Sequential IDs are {prefix}{n:03d} — behavior with two prefixes is not pinned by any test. The api_v1_test.go fixture uses id_type: short for the same shape, so the prototype is exercising an unclaimed combination.'
severity: significant
reason: 'Prototype-only. The dogfood project (tickets/) and the unit-test fixture both use id_type: short for multi-prefix types, which is the intended supported configuration and is fully tested. Sequential + multi-prefix is an interesting edge case but pinning it down requires either (a) confirming the workspace.GenerateNextID semantics for that combo or (b) deciding to disallow it at the loader level; that work belongs in a separate ticket scoped to id_type semantics. The prototype works for its dogfooding purpose either way.'
status: deferred
---
