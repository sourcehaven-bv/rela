---
id: RR-KO20QS
type: review-response
title: 'Output writer rebinding drops unmatched-verified flag (unmatched_principal: reject bypass)'
finding: 'internal/appbuild/realtime_outputs.go:65-76: as() restamps the tool and rebinds through identityBinderFor -> acl.BindRequest -> ResolveDirect. The data-entry Opener (internal/access/opener.go resolveIdentity) marks the Identity WithVerifiedAssertion, and Resolve sets unmatched. Both are lost on the rebind, so Request.UnmatchedVerified() is false and the unmatched_principal: reject gate (internal/acl/declarative.go:203) no longer denies. A verified principal with no user entity, whose own HTTP writes are rejected, can post a message and have the set/create written as them. Retries (VerifiedFrom from the mark) lose the flag as well. The promoter (realtime_promote.go) writes on the request ctx and has no such gap. Fix: derive the tool-stamped Request from the request''s existing Identity, for example with an acl helper that swaps the principal''s tool and keeps unmatched and verified, instead of resolving again. Store the unmatched-verified bit in the mark and refuse retries for it under reject. Add a test with unmatched_principal: reject and an unmatched verified poster.'
severity: critical
resolution: acl.RetoolRequest keeps the unmatched-verified state inline; the mark stores Unmatched/UserEntity and the retry resolves with WithVerifiedAssertion. TestOutputWriter_UnmatchedRejected covers pre-check, inline write and retry.
status: addressed
---
