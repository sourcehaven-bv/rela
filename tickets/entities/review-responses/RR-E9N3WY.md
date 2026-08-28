---
id: RR-E9N3WY
type: review-response
title: verifiedPrincipal doc says 'only constructor that can populate them' — UnmarshalJSON also does
finding: The verifiedPrincipal godoc called principal.Verified 'the only constructor that can populate them'. principal.Principal.UnmarshalJSON also populates the unexported claim fields (from the audit wire format). The intent was correct — no composite literal can forge claims, and Verified is the only request-path constructor — but the absolute phrasing is slightly imprecise for a load-bearing security comment.
severity: nit
resolution: Reworded to 'the only constructor that populates them from request-path input', explicitly noting UnmarshalJSON reads them back only from a record this process already wrote and verified. Precision matters here because the comment is the human-readable statement of the trust boundary.
status: addressed
---

## Finding

`verifiedPrincipal`'s godoc said `principal.Verified` is "the only constructor
that can populate them." `principal.Principal.UnmarshalJSON` also populates the
unexported claim fields — from the audit wire format. The comment's *intent* (no
composite literal can forge claims; `Verified` is the only request-path
constructor) is correct, but the absolute phrasing is imprecise for a
load-bearing security comment.

## Resolution

Reworded to "the only constructor that populates them from request-path input,"
noting `UnmarshalJSON` reads them back only from a record this process already
wrote and verified.
