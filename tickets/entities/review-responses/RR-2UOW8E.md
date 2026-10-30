---
id: RR-2UOW8E
type: review-response
title: columnTitle ignores face
finding: models.ts:706 titles a face column without a label "column", while columnSource on the next function says "face". The Configure row title, the Remove button label ("Remove column") and the change review then name it "column". Add c.face ? 'face' to the fallback chain, in line with the busy change.
severity: nit
resolution: ADDRESSED columnTitle falls back to "face" for a face column; models.test.ts asserts it.
status: addressed
---
