---
id: RR-RPH6CC
type: review-response
title: facedTicketApp mutates the metamodel after the app is built
finding: Anything derived from the schema at build time would not see the faces, and the affordance test no longer goes through metamodel.Parse.
severity: minor
reason: Same pattern as facedCreateApp in createworld_test.go; the handlers read faces from State().Meta per request and all face tests pass on it. The metamodel is not under test here.
status: wont-fix
---
