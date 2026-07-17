---
id: BUG-K6FEVB
type: bug
title: Relation write bypasses ACL (incl. --read-only) when peer entity does not exist
description: |-
    In entitymanager.CreateRelation (manager.go:684-692) the peer-existence lookups run BEFORE the ACL check, so a relation to a non-existent peer returns a soft "target/source entity not found" error rather than a ForbiddenError. The data-entry reconciler writeCreateRelation (relations_modern.go:364-373) classifies that as a DEC-HWZHA soft condition and re-issues the write DIRECTLY against the ungated store.Store, bypassing entitymanager/ACL/audit. Under `rela-server --read-only` (implemented solely as ReadOnlyACL) a relations-only PATCH adding an edge to a non-existent peer returns 200 and the edge persists on disk with no audit record. Reproduced live: .ignored/security-audit/acl-run-1/demos/demo_a.sh. Severity HIGH.

    Fix (decided): authorize FIRST in CreateRelation/UpdateRelation/DeleteRelation (independent of peer existence), and make a missing peer a hard 422 (not a silent soft-warn) so the UI can report the unresolved reference — removing the ungated direct-store fallback entirely. This deliberately reverses the DEC-HWZHA soft-condition treatment for this one case; document why (authz + user feedback).
priority: high
why1: A relations-only PATCH adding an edge to a non-existent peer lands a write even under --read-only, because the data-entry reconciler falls back to a direct store.CreateRelation on a 'soft' error, a path that never consults the ACL.
why2: The ungated direct-store fallback exists because DEC-HWZHA's 'tolerate temporarily invalid data' policy treats a missing target as a warning, not a rejection; entitymanager rejects dangling relations, so honoring warn-don't-reject was done by bypassing entitymanager (relations_modern.go:346).
why3: Bypassing entitymanager also dropped authorization because in entitymanager the authz concern and the existence-validation concern are entangled in one call sequence with existence checked first — the only way to skip the existence rejection was to skip the whole method, and authz came along for the ride.
why4: DEC-HWZHA was reasoned about purely as a validation policy (hard-400/422 vs soft-warn); the authorization boundary was never part of that mental model, so nobody asked whether the soft-condition fallback preserved the authz invariant.
why5: 'Systemic: authorization is enforced by convention at each call site rather than by construction at a mandatory chokepoint, and no invariant test fails when a code path reaches the store without an ACL decision. --read-only is only ReadOnlyACL, so any un-gated store path silently defeats it. (Same class as prior BUG-JME1DI: conflict endpoints bypassed ACL/audit.)'
prevention: 'P1: authorize FIRST in every entitymanager write method, from inputs independent of peer/entity existence, so a soft not-found can never precede/skip authz. P4 (this bug''s automated measure): an integration test that enumerates every registered /api write route and asserts each produces a denied-write / no store mutation under ReadOnlyACL. P5: consider a store-level read-only guard so --read-only is defense-in-depth, not ACL-only.'
status: done
---

Fix implemented on branch `fix/acl-relation-write-bypass`. PR: (opening)

entitymanager CreateRelation/UpdateRelation/DeleteRelation now authorize BEFORE
the peer/relation existence lookups, so a denied write returns ForbiddenError
regardless of peer existence. The ungated direct-store fallback for a missing
peer is removed: a dangling-peer relation write now returns HTTP 422
(structuralError, code target_not_found) so the UI can report the unresolved
reference. The type-allowlist soft-warn path is preserved (safe now that ACL
runs first). Tests: TestReadOnlyACL_DanglingPeerRelationWrite_Refused,
TestDanglingPeerRelationWrite_AllowedACL_422,
TestReadOnlyACL_EveryWriteRoute_DeniesAndDoesNotMutate (P4 =
AM-acl-readonly-write-route-invariant),
TestManager_RelationWrite_AuthorizesBeforePeerExistence. build / go test -race /
arch-lint / golangci-lint green; demo_a flips to NOT-REPRODUCED.
