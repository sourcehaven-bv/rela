---
id: RR-61XMGFK
type: review-response
title: A login is looked up as an entity id
finding: Without principal_property a principal is a login such as alice; if an entity with that id exists the timeline would show its title for an unrelated login.
severity: minor
resolution: Actors are named only when the policy maps principals onto persons (pilePersonType) and the header's type is that person type; the no-mapping case is in the table test.
status: addressed
---
