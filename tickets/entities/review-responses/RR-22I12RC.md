---
id: RR-22I12RC
type: review-response
title: 'Design: fill helpers should take typed selectors'
finding: A raw WHERE fragment parameter invites injection and broad row locks.
severity: minor
resolution: 'Plan changed: fill helpers take typed selectors built from package constants (row, family, incident relations, relation type, unhashed batch).'
status: addressed
---
