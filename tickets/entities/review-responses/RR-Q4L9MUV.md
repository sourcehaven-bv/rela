---
id: RR-Q4L9MUV
type: review-response
title: Semi Join assertion is broad
finding: NotContains Semi Join holds only because the explain test uses AllowAll.
severity: nit
resolution: The test only uses AllowAll. Scope the assertion to the text_hits node when the test is extended.
reason: The explain test only uses AllowAll today. Scoping the assertion to one plan node adds parsing for a case no test has.
status: wont-fix
---
