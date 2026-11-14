---
id: RR-RODNPI9
type: review-response
title: DELETE answering 404 counted in the delete window
finding: A 404 to the DELETE counted as Deleted and used up the 24 h window.
severity: minor
resolution: A 404 forgets the row without counting it, as a 404 on the fetch does.
status: addressed
---
