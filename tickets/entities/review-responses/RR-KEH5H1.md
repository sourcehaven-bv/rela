---
id: RR-KEH5H1
type: review-response
title: Principal.Sanitized and access.Opener rebuild Principal and would drop AssertedBy
finding: Section 1.5 adds AssertedBy to Equal, ShareKey and JSON only. principal.Sanitized builds a new Principal field by field, and audit/filesystem.go:216 runs it before every write, so the "audit carries asserted_by" pin (AC13) fails without an error. access/opener.go:174 (resolvePrincipal) and Restamp also rebuild through VerifiedFrom and would drop it. List every rebuild site in the plan, or see the next finding.
severity: significant
resolution: Per Jeroen the marker is carried on ctx, not as a Principal field, so no rebuild site can drop it.
status: addressed
---
