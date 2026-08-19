---
id: RR-R7TFQ2
type: review-response
title: A7 finding text still names requires_permission as the only consumer, so the remediation is wrong for UI-gated permissions
finding: 'internal/aclaudit/tier_a.go:220-224 — the whole premise of these commits is that requires_permission is NOT the only place a permission is referenced, yet the message an operator reads is unchanged: ''which no role_relations.requires_permission references; the permission is dead'' with ''fix: reference X in a requires_permission gate, or remove it''. After the fix that Detail is still literally true but is no longer the REASON the permission is dead, and the Fix is the wrong remediation for a report/command/nav permission — those are gated by `permission:` in data-entry.yaml, not by a relation gate. The original bug report called out this exact Fix string as ''actively harmful''; the false positive is fixed but the misleading remediation that made it harmful survives. docs/acl-security.md:74-78 describes A7''s scope in the same narrow terms and was not updated.'
severity: significant
status: open
---

## Fix

Reword Detail and Fix to match the widened check — name all three consumer
classes (relation gate, data-entry UI gate, rela built-in) so the operator knows
where a live reference could legitimately come from, and offer both remediation
routes.

Update `docs/acl-security.md` in the same change so the prose and the emitted
message agree.

Severity significant: an operator following the current hint on a UI-gated
permission still removes a working grant. The blast radius is smaller than
before (they now only see the message when the permission really is
unreferenced), but the advice remains wrong for the most common non-relation
case.
