---
id: RR-DMZ5NA
type: review-response
title: Malformed tail reported as undeclared face
finding: validTail's message claimed a declaration problem for a grammar error.
severity: nit
resolution: The message now says the tail is not a valid face name; the ErrFaceNotDeclared sentinel stays for the HTTP mapping.
status: addressed
---
