---
id: RR-BZEMFJS
type: review-response
title: New trigger may be placed before existing ones
finding: keysInOrder ignored new keys, so a new trigger could run before a scripted one.
severity: significant
resolution: 'newKeysLast refuses a new key before any kept key in an ordered mapping. Test: add a trigger before the others.'
status: addressed
---
