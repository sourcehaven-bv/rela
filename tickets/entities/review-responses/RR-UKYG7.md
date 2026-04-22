---
id: RR-UKYG7
type: review-response
title: entity-api filter test passes vacuously on empty result
finding: '`for (const e of page.data) { expect(e.properties.status).toBe(''draft'') }` passes with zero teeth if the backend returns []. Missing a length>0 guard.'
severity: critical
resolution: entity-api list-filter test now seeds a draft and a done feature, asserts length>=1, and verifies the draft appears while the done doesn't.
status: addressed
---
