---
id: BUG-ZWTDH9
type: bug
title: Sync PUT authorizes UPDATE against caller-supplied type, not stored type (cross-type privilege escalation)
description: |-
    PUT /api/sync/entities/{id} -> putEntity (sync_handlers.go:129) builds the entity from body.type, and ApplyEntity (apply.go:96-99) authorizes acl.WriteRequest{Op:OpUpdate, Subject:EntitySubject{Type:e.Type}} using that caller-supplied type. ApplyEntity calls GetEntity(e.ID) only for op-selection and DISCARDS the result (apply.go:90) — it never reads or compares the stored type. The update grant is matched against the claimed type (authz_write.go -> policy.go grantsVerb). A principal with update:[note] who can READ a secret-typed target can PUT {"type":"note",...} to overwrite and re-type it; fsstore writes entities/notes/<id>.md and leaves the original entities/secrets/<id>.md, corrupting the store; audit misattributes the type. ValidateEntity's only guard (ID-prefix) is skipped for id_type:manual. Contrast: handleV1UpdateEntity + attachmentWritePreflight reject entity.Type != typeName and authorize against the STORED type. Reproduced live: demos/demo_b.sh. Severity HIGH.

    Fix (decided): on update, authorize+validate against the STORED type and hard-reject (422/409) a body type that differs — matching the v1 'type is immutable on update' contract.
priority: high
why1: An attacker with update:[note] can mutate a secret-typed entity because the ACL subject's Type is taken from the request body and the update grant is checked against that claimed type, not the resource's real type.
why2: ApplyEntity is a generic create-OR-update upsert that treats the incoming entity as the source of truth for all fields including type, and authorizes 'can this principal write an entity of type X' before looking at what is on disk.
why3: It already reads the entity via GetEntity(e.ID) but discards it (apply.go:90) — used only to pick create-vs-update; the stored type was one line away but never wired into the authz subject. The two uses of 'does it exist' (op selection vs authz subject) were never connected.
why4: 'The sync API and the v1 PATCH path were built at different times with different contracts: v1 PATCH has no type field (type structurally immutable); sync accepts a full entity body including type. No shared ''type is immutable on update'' invariant is enforced across both, and sync''s stance was never audited against the ACL.'
why5: 'Systemic: the authorization subject is assembled from client-controlled input at each call site, with no single chokepoint binding the ACL subject to the STORED resource. Every new write path re-decides what it authorizes against, and no test asserts ''authz subject type == the resource actually mutated''.'
prevention: 'P2: derive Subject.Type from the loaded entity on update and reject a differing body type, via one helper used by every write entry point. P4 (automated measure): an integration test asserting, for every entity-write entry point (v1 PATCH, sync PUT, ...), that the ACL subject type equals the stored entity''s type and a mismatched body type is rejected.'
status: review
---

Fix implemented on branch `fix/acl-sync-put-type-confusion` (commit 1c5e94eb).
PR: (see below)

ApplyEntity now captures the GetEntity result (previously discarded), rejects a
body type != stored type with new sentinel ErrTypeImmutable -> HTTP 422
(type_immutable), and binds the ACL subject to the stored type. ApplyRelation
confirmed not vulnerable (resolves FromType from a store read; sync relation
body carries no endpoint types). Tests:
TestApplyEntity_RejectsTypeChangeOnUpdate, TestSyncPut_RejectsTypeChangeOnUpdate
(+ same-type regressions), TestWriteSubjectTypeInvariant (P4 =
AM-acl-write-subject-type-invariant). build / go test -race / arch-lint green;
demo_b flips to NOT-REPRODUCED.
