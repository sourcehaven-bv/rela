---
id: RR-SRJLHON
type: review-response
title: Second write loses copy provenance
finding: Writing automation properties in a second write after the copy drops the copy's provenance and adds an extra audit record.
severity: significant
resolution: Automation properties are folded into the copy's own write before the transaction; one audit record.
status: addressed
---
