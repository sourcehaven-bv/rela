---
id: BUG-ABXMAV
type: bug
title: /relations endpoints leak hidden-neighbor type + edge meta past the read gate
description: |-
    handleV1EntityRelations (api_v1.go:1255-1327) and handleV1GetRelationType (api_v1.go:1378-1433) gate only the SOURCE entity via gateReadOrNotFound, then emit each neighbor's id, type (via the deliberately-ungated entityReader.entityType, entityreader.go:37-42), and edge meta (edge.Properties) with NO per-neighbor read gate. The bare neighbor id is an already-accepted leak (per-entity GET ships an IDs-only relations map, acl_get_test.go); the NOVEL leak is neighbor type + edge meta, plus a per-relation-type enumeration oracle. Contrast: the list handler (handleV1ListEntities) computes visibleRelationIDs and drops hidden neighbors (RR-HJV8CP, pinned by acl_list_neighbor_test.go); ?include= uses filterVisibleIncludes; the dedicated /relations endpoints do neither and have no ACL regression test. Reproduced live (NOTE-H is 404 on direct GET but enumerable via /notes/NOTE-S/relations): demos/demo_c.sh. Severity HIGH.

    Fix (decided): compute the visible neighbor set (visibleRelationIDs) in both handlers and skip any edge whose peer is not visible, including the ungated entityType lookup for hidden peers — mirroring the list path.
priority: high
why1: A hidden neighbor's type and edge meta leak because these handlers apply the read gate to the source entity only and emit neighbor fields without checking each neighbor's readability.
why2: 'Read filtering was retrofitted onto rela per-endpoint: the list and ?include= paths got visibleRelationIDs/filterVisibleIncludes; the dedicated /relations endpoints were a separate handler the retrofit missed.'
why3: The fix (RR-HJV8CP) was framed and tested against the surface where the leak was noticed (the list), not against 'every surface that emits a neighbor'. The invariant lived in a test bound to one handler, not a shared function every neighbor-emitting path must call.
why4: 'Neighbor gating is a per-handler responsibility rather than a structural one: entityType is deliberately ungated and neighbor visibility depends on each handler remembering to call the filter. There is no single ''serialize a neighbor for the wire'' chokepoint that gates by construction.'
why5: 'Systemic: read filtering is applied at leaf handlers, not at a mandatory serialization boundary, so ''did this response leak an unreadable entity?'' is not enforced by construction and not covered by a cross-endpoint test. A new or overlooked neighbor-emitting endpoint fails open by default.'
prevention: 'P3: route every neighbor-to-wire emission through one function that takes the read gate and drops unreadable peers (id, type, meta), so new endpoints inherit the gate. P4 (automated measure): an integration test enumerating every neighbor-emitting read endpoint (list, ?include=, /relations, /relations/{type}, single-GET) that asserts a hidden neighbor never appears (id/type/meta).'
status: review
---

Fix implemented on branch `fix/acl-relations-neighbor-leak` (commit 0ce4f18d).
PR: https://github.com/sourcehaven-bv/rela/pull/1113

Both `/relations` handlers now gate neighbors via `visibleRelationIDs` (same
helper as the list path), skipping hidden peers before the ungated `entityType`
lookup. Tests: `TestACLRelations_HiddenNeighborExcluded` (reproduction,
failing-first confirmed) + `TestACLNeighborReadLeakInvariant` (P4 cross-endpoint
invariant = AM-acl-neighbor-read-leak-invariant). build/test/arch-lint green;
demo_c flips to NOT-REPRODUCED.
