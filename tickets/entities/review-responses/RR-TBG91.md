---
id: RR-TBG91
type: review-response
title: Two divergent scalarPredicateType adapters (predicatefns vs affordances)
finding: 'predicatefns/env.go scalarPredicateType maps integer->IntType, date->DateTypeWithLayout. affordances/env.go has its OWN scalarPredicateType mapping integer->NumberType, date->StringType, with matching Number/String binders. Both internally consistent so nothing breaks today, but same name + same purpose + opposite output. When someone wires predicatefns.EntityRecordType into the affordances eval path (the obvious Phase-2 consolidation), declared Int/Date fields meet Number/String bound values and runtimeTypeAccepts fails at eval. Phase-1 action: cross-document the divergence (note in predicatefns/env.go that affordances has NOT migrated and the binders are not interchangeable). Full merge is the Phase-2 consolidation.'
severity: significant
resolution: 'EntityRecordType godoc (predicatefns/env.go) now explicitly documents that it is NOT interchangeable with affordances'' scalarPredicateType: affordances predates Int/Date and maps integer->Number, date->String with matching binders, so feeding this function''s RecordType into the affordances eval path would fail the runtime type check. Notes that unifying the adapters + migrating the affordances binder to NewInt/NewDate is the Phase-2 consolidation. Full merge deliberately deferred to Phase 2 (can''t merge until affordances adopts the typed values).'
status: addressed
---
