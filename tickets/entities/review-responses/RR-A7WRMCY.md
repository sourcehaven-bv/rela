---
id: RR-A7WRMCY
type: review-response
title: 'Trigger ACL check: schema edit judges unchanged triggers without the editor'
finding: A schema edit that widens what an unchanged trigger fires on was compared without the editor's own rights.
severity: minor
resolution: The before/after comparison for unchanged triggers on a schema edit now includes the editor rules.
status: addressed
---
