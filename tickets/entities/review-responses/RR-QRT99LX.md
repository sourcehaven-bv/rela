---
id: RR-QRT99LX
type: review-response
title: 'Design: ScopeOwner scans all relations per call'
finding: 'The write path should not call ScopeOwner per edge.'
severity: nit
resolution: 'The write path uses an index compiled once in entitymanager.New.'
status: addressed
---
