---
id: RR-WQ05Z
type: review-response
title: FSStore.bytes field name shadows stdlib bytes package
finding: FSStore.bytes field name clashes with stdlib bytes package (imported in several fsstore files including markdown.go). Go allows it; readability suffers. fsstore.go:167-169 uses same identifier as local variable. Minor reviewer friction and possible autocomplete confusion.
severity: nit
reason: Field rename (bytes -> data/byteFS/storeFS) touches ~20 call sites in fsstore. Given the narrowing of FSStore.fs to s.dirs + s.rawReader already went in this round, deferring the bytes rename to a follow-up keeps this PR focused on the encryption refactor. Cosmetic nit only.
status: deferred
---
