---
id: RR-RVJ2PH2
type: review-response
title: Actor is not rewritten when a person is renamed
finding: RenameEntity and RenameOwner rewrite refs and owners but not Actor so a renamed person's notifications lose their cause.
severity: minor
resolution: RenameOwner now rewrites the actor of every item in kvpiles and pgpiles. Store contract updated; conformance case 'rename owner moves the actor on every pile' passes on both (pg run against a real database).
status: addressed
---
