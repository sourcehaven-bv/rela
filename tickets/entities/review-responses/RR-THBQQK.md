---
id: RR-THBQQK
type: review-response
title: 'permission: on entity-anchored documents was entirely untested'
finding: 'gateDocumentPermission was added to the entity-anchored path (api_v1.go:2124) with zero test coverage — every test in standalone_document_handler_test.go exercised the standalone path. That is the branch where the interaction is non-trivial: `permission:` composes with gateReadOrNotFound, and a code comment asserts it ''narrows, never widens'' with nothing verifying it. Separately, the gate was inserted into a sequence whose ordering a pre-existing comment calls load-bearing (ACL gates must precede the entity_type-mismatch 400, or a denied caller gets a 400 that reveals the target entity''s type), and no test pinned that order.'
severity: critical
resolution: |-
    Added TestAnchoredDocument_PermissionGate with three principals: holds-permission-and-can-read (200), can-read-but-no-permission (403), and holds-permission-but-cannot-read-the-entity (404 — proving the permission narrows and never widens; the entity gate fires first because entity existence IS secret). Added TestAnchoredDocument_GateOrderingNoTypeOracle, which probes a right-type and a wrong-type id as a principal denied BOTH entity types and asserts the two responses are identical modulo `instance`; a 400 from either means the entity gate moved below the type-mismatch check.

    Revised after the config-secrecy correction (RR-E8Z1MR): the permission deny is now 403 rather than 404, and the ordering test was rewritten around a principal denied the entity read rather than one denied the document permission — the type oracle it guards is about entity existence, which is genuinely secret, not about a config key, which is not.
status: addressed
---
