---
id: RR-V49F
type: review-response
title: Method dispatcher consults URL shape only — pin in security guide
finding: 'handleV1SingleEntity (api_v1.go:706-720) returns 405 with Allow header for unknown methods, BEFORE any per-method handler runs and therefore before Visible probes. Today this depends on URL pattern only, so a hidden entity behaves identically to a visible one for OPTIONS/405 — correct. But unsupported: a future maintainer could accidentally consult entity existence in the dispatcher. Pin in GUIDE-acl-security: ''The method dispatcher MUST NOT consult entity existence — dispatch on URL shape only; per-method handler probes Visible.'' Backed by a test: OPTIONS to a hidden entity returns same Allow header as OPTIONS to a visible one.'
severity: minor
resolution: 'GUIDE-acl-security read-path section already documents "the method dispatcher MUST NOT consult entity existence — dispatch on URL shape only; per-method handler probes PermitsRead." Code matches: handleV1SingleEntity returns 405 from URL pattern alone. Explicit OPTIONS-on-hidden test deferred to TKT-VMD8 docs follow-up since OPTIONS handling isn''t expanded in this PR.'
status: addressed
---
