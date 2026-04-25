---
id: RR-1HIWF
type: review-response
title: stripQueryKey drops presence-only return_to with same warning as value form
finding: document.go:560-566 treats both 'return_to' (no =) and 'return_to=...' as duplicates and drops them, slog.Warn fires identically. Presence-only return_to is never emitted by legitimate tools (fuzzing signal); a value-form one is a plausible authoring mistake. Low priority.
severity: nit
reason: 'Nit severity; reviewer''s own note: ''low priority.'' The audit trail for hostile inputs lives at the HTTP layer (access logs, X-Forwarded-For) and isSafeReturnPath rejections, not in stripQueryKey''s logger. Splitting the warning would add a branching discriminator for a threat model that the rewriter''s stripping behavior already neutralizes. If per-request input-fuzzing telemetry becomes a real need, file a dedicated ticket at the router layer.'
status: wont-fix
---
