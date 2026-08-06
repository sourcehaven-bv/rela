---
id: RR-THBQQK
type: review-response
title: 'permission: on entity-anchored documents was entirely untested'
finding: 'gateDocumentPermission was added to the entity-anchored path (api_v1.go:2124) with zero test coverage — every test in standalone_document_handler_test.go exercised the standalone path. That is the branch where the interaction is non-trivial: `permission:` composes with gateReadOrNotFound, and a code comment asserts it ''narrows, never widens'' with nothing verifying it. Separately, the gate was inserted into a sequence whose ordering a pre-existing comment calls load-bearing (ACL gates must precede the entity_type-mismatch 400, or a denied caller gets a 400 that reveals the target entity''s type), and no test pinned that order.'
severity: critical
resolution: 'Added TestAnchoredDocument_PermissionGate with three principals: holds-permission-and-can-read (200), can-read-but-no-permission (404, renderer not invoked), and holds-permission-but-cannot-read-the-entity (404 — proving the permission cannot widen entity access). Added TestAnchoredDocument_GateOrderingNoTypeOracle, which probes a right-type and a wrong-type id as a denied principal and asserts both responses are identical modulo `instance`; a 400 from either means a gate moved below the type check. Two initial failures were my test bugs (the fake document service builds its own store, so entities need seeding there too; and my first oracle assertion compared `instance`, which necessarily differs since it echoes the requested URL).'
status: addressed
---
