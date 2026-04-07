---
id: RR-JV6M
type: review-response
title: 'AC #15 contains unfinished editing residue'
finding: 'Acceptance criterion #15 in PLAN-FOOU starts with ''The ai global does not exist when no client is configured...'' and then immediately self-corrects with ''Wait, no: per decision 4, ai global is *always* registered''. The criterion title and body now contradict each other; an automated check or future reader will assert the wrong invariant. This is unfinished editing left in the document and is also a tell that other parts may be sloppy.'
severity: significant
resolution: 'Acceptance criteria list completely rewritten in PLAN-FOOU. The contradicting AC is now AC #37: ''The ai global is always registered''. No more ''Wait, no'' residue.'
status: addressed
---
