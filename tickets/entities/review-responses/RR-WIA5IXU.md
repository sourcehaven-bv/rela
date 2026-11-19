---
id: RR-WIA5IXU
type: review-response
title: 'Migration cleanup could drop a redundant widget: text override'
finding: 'A data-entry config that sets widget: text on a field whose property now has display.link keeps the text widget there. A migration could remove the redundant override.'
severity: minor
resolution: 'Deferred: no migration added.'
reason: 'An explicit widget: override is an operator choice and still works. No config in the repo or in the harbourmaster example sets one on a link property.'
status: deferred
---
