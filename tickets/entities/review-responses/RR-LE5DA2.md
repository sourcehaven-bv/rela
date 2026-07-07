---
id: RR-LE5DA2
type: review-response
title: Version-row attribution inherits the audit log's proxy-trust model (spoofable without a verifying proxy)
finding: 'principal_user/tool are captured from principal.From(ctx). Under HeaderPrincipalResolver without a trusted proxy, a caller sets X-Forwarded-User and forges attribution (router.go:313-334; sanitize only strips control chars, doesn''t authenticate). The version table PERSISTS this forgeable attribution into a durable audit-like history. Good news: Lua cannot rewrite principal (read-only, guarded by audit_spoofing_test.go) and triggered_by is server-controlled — so the only vector is the untrusted-proxy misconfiguration, identical to the audit log. Fix: read principal ONLY from ctx (never a request body/caller field); document that version attribution has the same trust model as the audit log and recommend JWTPrincipalResolver where attribution is relied upon. No code fix beyond that.'
severity: minor
resolution: 'Addressed by design + documentation: the version writer reads principal ONLY from ctx (delete-capture) or correlates via the audit log (swept versions) — never from a caller-supplied field. This is the SAME trust model as the existing audit log; docs recommend JWTPrincipalResolver where attribution accountability matters. Lua-side principal rewrite is already blocked (audit_spoofing_test.go); triggered_by is server-controlled. No code change beyond reading from ctx.'
status: addressed
---
