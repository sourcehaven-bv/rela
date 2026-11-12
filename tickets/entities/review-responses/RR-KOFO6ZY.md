---
id: RR-KOFO6ZY
type: review-response
title: Link refuses a ledger row in any state
finding: 'Cranky 6: a detached or dropped ledger row for the address or the local id blocks a link with an unclear message.'
severity: nit
resolution: 'Kept on purpose: the ledger row is the sync''s record of that row and a link must not overwrite it. The error now says the ledger already holds the row and the doc comment states the rule.'
status: addressed
---
