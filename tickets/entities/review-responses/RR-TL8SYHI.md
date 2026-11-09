---
id: RR-TL8SYHI
type: review-response
title: goneEntity reports on raw-read rows
finding: goneEntity read the local row raw and reported its id, edited field and relations even when the puller cannot read it.
severity: significant
resolution: 'goneEntity checks the gate first; a hidden row is detached with a generic kept issue and no local id. Test: TestRun_HiddenLocalRowsAreNotNamed/gone_entity.'
status: addressed
---
