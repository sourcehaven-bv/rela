---
id: RR-RUD546
type: review-response
title: Large cross-block ranges hide highlights of comments inside them
finding: selectNonOverlapping kept the range starting first.
severity: minor
resolution: 'When one range lies inside another the inner one is kept; partial overlaps still keep the first. Test: ''keeps a short comment inside a long cross-block one''; docs note it.'
status: addressed
---
