---
id: RR-L1MY0N
type: review-response
title: No public ID-preserving upsert; explicit IDs rejected for short/sequential types
finding: 'Sync requires re-creating a record on the peer WITH ITS EXISTING ID (the id is the cross-side identity for the manifest/index). But verified in code: CreateEntity rejects an explicit ID when the type is not manual-id (manager.go:334-343, core.go:45-53) — and ''short'' is the DEFAULT id_type (metamodel/types.go:200-205). So a locally auto-assigned id like REQ-a3f8 CANNOT be created on the server via the public CreateEntity. There is NO public upsert-with-explicit-id method on the EntityManager interface. The internal upsertEntity (core.go:268-280) preserves the id and does create-then-update-on-conflict, but it is an unexported free function that bypasses the ACL/audit/validation framing of the public methods. Reaching past the manager loses audit/ACL/automation (violates the project''s ''all writes go through entitymanager'' rule + the ticket''s own acceptance criterion #5). The plan needs a DECISION: add a new public manager method (e.g. ApplyEntity / Upsert) that preserves the id AND keeps ACL/audit/automation, used by both push (server) and pull (local) apply paths. This is load-bearing — without it, sync literally cannot create records on the peer.'
severity: critical
resolution: 'Plan updated (Approach §5): add a NEW public entitymanager method ApplyEntity/ApplyRelation that preserves the supplied id, does create-or-update, and keeps ACL + audit + validation (modeled on internal upsertEntity core.go:268-280 but with full public framing). Used by both server push-apply and local pull-apply. Added to Files-to-modify and to the Risk list (HIGH: must keep framing while suppressing automation).'
status: addressed
---
