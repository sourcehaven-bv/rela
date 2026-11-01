---
id: RR-GU22EQZ
type: review-response
title: Rules still run after the Validations section is full, with no visible gain
finding: 'Design review: the plan said running every rule after the cap keeps later script and load errors visible. capIssues cuts every issue past index 100, so those errors never reach the wire. The extra rules cost about 0.6 s on atlas for nothing the user sees.'
severity: significant
reason: 'The user chose complete reporting over the saved time before this finding was known. The implementation keeps that choice and the output is byte-identical to develop. The finding goes back to the user with two options: give script and load errors their own cap, or stop running rules once the section is full.'
status: deferred
---
