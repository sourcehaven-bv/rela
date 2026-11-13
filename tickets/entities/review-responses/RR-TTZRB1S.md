---
id: RR-TTZRB1S
type: review-response
title: Object lists reach code that assumes a string list
finding: 'messageRecordType types every list as list<string>; PredicateValue drops non-string items so a when: on an object list silently sees []; presets, buttons and describe assume string lists.'
severity: significant
resolution: 'Plan: every p.List site is listed and handled. Object list properties are refused at load in when: environments, presets and triggers for v1; describe and CLI render them explicitly.'
status: addressed
---
