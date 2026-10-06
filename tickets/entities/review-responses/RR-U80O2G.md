---
id: RR-U80O2G
type: review-response
title: Floating comment in entityref.go
finding: entityref.go opens with a comment block about the address grammar that is attached to no declaration.
severity: nit
reason: It is a file-level note on the address grammar that several functions below rely on. Go allows it and commentlint does not flag it. Attaching it to one function would suggest it is about that function only.
status: wont-fix
---
