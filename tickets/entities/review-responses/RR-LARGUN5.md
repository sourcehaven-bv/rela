---
id: RR-LARGUN5
type: review-response
title: not_run counted against ranges the server never tried
finding: The server answers not_run for every range after its deadline; counting all of them splits cheap ranges and can end in ErrBudget.
severity: minor
resolution: Only the first not_run in an answer is counted; the rest are asked again unchanged.
status: addressed
---
