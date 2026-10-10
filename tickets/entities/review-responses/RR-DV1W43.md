---
id: RR-DV1W43
type: review-response
title: '[security] Comment posted between dropThreads and DeleteFamily survived the delete'
finding: '[security] The migration lock does not hold off live comment writes, so a comment posted while the row was still readable outlived it and passed to the next entity reusing the id.'
severity: minor
resolution: dropEntitiesStep calls dropThreads again after a successful DeleteFamily; the doc on dropThreads explains both calls.
status: addressed
---
