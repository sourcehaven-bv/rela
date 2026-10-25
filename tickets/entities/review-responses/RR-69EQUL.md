---
id: RR-69EQUL
type: review-response
title: Label-drop comment asserts an unverified cause
finding: security.yml comment says GitHub silently drops labels on app-created issues; the cause was not confirmed.
severity: minor
resolution: 'Comment now states the observed fact: app-created issues have no labeled timeline events (checked #1986) while GITHUB_TOKEN issues do (#1922).'
status: addressed
---
