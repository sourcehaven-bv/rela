---
id: RR-4O6E
type: review-response
title: Dry-run as a verb collides with audit; OpDeniedWrite would fire on every keystroke
finding: denyAffordance (used by the create gate) records an OpDeniedWrite audit row on every denial. With live re-derivation, a user typing into a read-only field would emit a denied-write audit record per debounce tick — flooding the audit log with non-events (the user never committed). The dry-run path must compute verdicts WITHOUT emitting audit. This means the dry-run cannot simply call the existing validateFieldWrite+denyAffordance flow; it needs a verdict-only path that returns _fields/warnings and never touches auditSink. Re-confirm at commit (real create) is where the single audit row belongs. Plan must separate 'compute verdicts' from 'enforce + audit'.
severity: significant
resolution: 'Plan updated: dry-run uses a verdict-only path that returns _fields/warnings and never calls denyAffordance/auditSink. The ''compute verdicts'' step (shared with serializer) is split from ''enforce + audit'' (commit only). The single OpDeniedWrite audit row belongs to the real commit, not the keystroke-level dry-run.'
status: addressed
---
