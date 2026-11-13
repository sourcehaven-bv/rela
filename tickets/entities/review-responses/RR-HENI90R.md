---
id: RR-HENI90R
type: review-response
title: Performable repeats the state lookup and branch scan
finding: Performable checks the state and decideMove reads it again and rescans the exits per target.
severity: nit
resolution: 'Won''t fix: the read is bounded to the out-edges of one field, and one entry point that owns the legality checks is the drift guard; a second entry point would be the thing that can drift.'
reason: The read is bounded to the out-edges of one field; one entry point that owns the legality checks is the drift guard, and a second entry point is what could drift.
status: wont-fix
---
