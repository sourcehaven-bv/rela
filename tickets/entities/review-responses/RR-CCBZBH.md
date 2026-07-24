---
id: RR-CCBZBH
type: review-response
title: No Bind at any script entry point — 105 membership-graph walks where 1 suffices
finding: 'DeclarativeGate''s godoc states the WIRING REQUIREMENT (RR-MXKD2O): open ONE acl.Request per logical operation and attach it via Bind. No production code calls Bind — only the conformance suite. Reviewer measured 5 list_entities over 20 rows: 105 membership-graph walks without Bind, 1 with. That is ~21 per call (one gate probe + one redactor probe per row), each opening a fresh Request and re-walking member-of. On a scheduled LLM job over thousands of entities this is the difference between a gate and a stall; the same godoc also warns the row-gate and redactor may evaluate against DIFFERENT ACL snapshots if a write lands mid-operation — a real window for a long-running scheduled job. Bind once at script-run entry.'
severity: significant
resolution: ScriptReader now takes an optional Binder (satisfied by DeclarativeGate) and binds ONCE per read call, so the row gate and the field redactor share one acl.Request instead of each opening its own. Binding at the ScriptReader rather than at each entry point means every script read is covered automatically, with no per-call-site discipline. Both production wiring sites pass the gate as binder. A bind failure is not swallowed — the call proceeds on the original ctx and the gate denies on its own terms, so a failure can never widen visibility.
status: addressed
---
