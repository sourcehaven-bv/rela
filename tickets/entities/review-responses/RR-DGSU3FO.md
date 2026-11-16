---
id: RR-DGSU3FO
type: review-response
title: llmWritable authorizes unused verbs per row
finding: computeActions authorizes update, delete and rename per row; a batch needs only update.
severity: minor
resolution: 'llmWritable authorizes only translateVerb(update). FieldVerdicts stays twice per row: hiddenProperties uses the visibility context, llmWritable the write one.'
status: addressed
---
