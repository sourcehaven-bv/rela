---
id: RR-TW2LUUG
type: review-response
title: 'Code: archguard misses an aliased elementnaive import'
finding: An aliased import of elementnaive evades the elementnaive.New match.
severity: nit
reason: The RangeFingerprint and ListRange selector match catches every call on the reader regardless of alias, so a read cannot happen unnoticed; resolving aliases adds complexity for no extra coverage.
status: wont-fix
---
