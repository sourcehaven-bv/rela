---
id: RR-HCV3XAG
type: review-response
title: Id placeholders accepted into any property type
finding: '{{subject.id}} and {{run.id}} skipped the type check for every target.'
severity: significant
resolution: 'An id is text: only a text property takes it. Test: id into date.'
status: addressed
---
