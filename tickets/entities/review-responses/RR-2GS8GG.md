---
id: RR-2GS8GG
type: review-response
title: List policy reads compiled List, not raw list
finding: 'internal/realtimecfg/validate.go:976 - the old code used rawProperty.List. A raw `list: true` on a non-string (refused at .list) now compiles to List=false, so a literal for it gets no list reason; an enum list literal passes the policy and is normalized as a scalar. Harmless because the file is already refused at .list; mention it in the comment if intended.'
severity: nit
resolution: "addressed\tKept the compiled List and added a comment that a raw list: true on a type that cannot be a list compiles to false, which is safe because the file is refused at its .list key."
status: addressed
---
