---
id: RR-BWDE
type: review-response
title: Wire-format amendment changed ACs without updating ticket; operator attribution dropped to slog.Debug
finding: Plan unilaterally amended TKT-9E57 AC3-AC5 (rule_kind=affordance:predicate, rule_id=<role>/<field>) to use TKT-G7N5's existing wire format. (1) Ticket text wasn't updated → reviewer comparing ticket to PR will fail it. (2) Role/predicate attribution moved to slog.Debug only — operators reading audit JSONL see rule_id=field-affordance:read-only:status for every deny with no way to attribute to which acl.yaml grant fired. Reproducing 'user X cannot edit field Y' becomes 'tail server log while user X clicks.'
severity: critical
resolution: 'Two-channel split: (1) Update ticket AC3-AC5 to match plan''s wire format before PR (added to implementation checklist). (2) AffordanceDenialError gains Attribution field (role=triager/grant=fields.ticket[0]/when=...) that flows into denyAffordance''s audit Summary but NOT into the external wire response. Audit consumers see full attribution; external clients see wire-stable rule_id. Pinned by test.'
status: addressed
---
