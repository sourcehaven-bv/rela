---
id: RR-6UG909K
type: review-response
title: 'Design: detach loses the link'
finding: Detach unset the ref and forgot the row, so a source grant restored later would create duplicates.
severity: significant
resolution: 'Plan: detach keeps the ref and a detached ledger row outside the diff, relinked when the row reappears; max_gone counts entities and relations in both modes.'
status: addressed
---
