---
id: RR-2I7JHJ
type: review-response
title: 'S2: EntityRecordType has no congruent binder; would fail at eval in production'
finding: 'Plan cited predicatefns.EntityRecordType as ready-made for phase 4. Its own godoc (predicatefns/env.go:29-38) says the opposite: it is NOT interchangeable with the affordances binder (RR-TBG91) which maps integer->Number and date->String. Three divergent metamodel-to-type adapters exist (predicatefns/env.go:56 affordances/env.go:108 conditionlint.go:113). Phase 4 must use the predicatefns one (only it has DateTypeWithLayout which days_between needs) and therefore needs a new binder that does not exist. Without it a date property binds as NewString and every days_between fails at Eval at write time in production - exactly the failure mode the plan claims to prevent.'
severity: significant
resolution: New predicatefns.BindEntity emitting NewInt/NewDate/NewString/NewBool congruent with EntityRecordType added to scope and Files. AC5 round-trips EVERY metamodel property type through EntityRecordType + BindEntity asserting the runtime type check passes - the highest-value test in the ticket.
status: addressed
---
