---
id: RR-4DAWMA
type: review-response
title: changes.ts rebuilds ColumnModel by hand
finding: 'changes.ts:500-510 repeats the columnModel literal from models.ts:689 and hard-codes face: false, so the next ColumnModel field needs a third edit and the two can drift (the review title for a face column already differs from what the page would compute if columnTitle learned face). Export columnModel from models.ts and call it here.'
severity: nit
resolution: ADDRESSED columnModel is exported from models.ts and changes.ts calls columnTitle(columnModel(item, i)).
status: addressed
---
