---
id: RR-DQPOT81
type: review-response
title: regexp_replace evaluated twice per row
finding: The stripped digits are computed in BETWEEN and in THEN.
severity: nit
reason: Both copies come from one Go variable, so they cannot drift; the cost is small.
status: wont-fix
---
