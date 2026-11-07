---
id: RR-4R6P2Z0
type: review-response
title: No test pins the reported disclosure itself
finding: 'The table cases pin the alias message but not the disclosure from the issue (subject: pers with {{subject.salary}} or entity.salary). A later reorder of checks could reopen it silently.'
severity: minor
resolution: 'TestParse_AliasSubjectReadsNoHiddenProperty loads subject: pers with a salary template and with a salary guard and requires the alias or the salary read to be refused. It fails without the fix.'
status: addressed
---
