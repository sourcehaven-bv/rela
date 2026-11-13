---
id: RR-ZU973VA
type: review-response
title: Literal zero keys and Limit counted as set
finding: 'EntityQuery{IDs: nil}, RelationQuery{From: ""}, Limit: 0 exempted the call although they do not bound it.'
severity: significant
resolution: 'litKeys and field assignments now treat nil, 0 and "" as unset; runtime-zero values are listed under Not caught. Tests: literal zero limit, literal empty key, key reset to nil.'
status: addressed
---
