---
id: RR-3TJMGN
type: review-response
title: fsstore skips files silently
finding: Undeclared type folders, unparsable names, duplicate ids across type folders and orphan attachment folders never reach the store API.
severity: significant
resolution: 'Plan updated: A file-level reconciliation walk lists every source file not copied, with its reason.'
status: addressed
---
