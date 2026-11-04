---
id: RR-E758LR1
type: review-response
title: Relation end ids are neither trimmed nor parsed
finding: An id with @face passed the gates and failed in the Tx with store text; a padded id answered target_not_found.
severity: minor
resolution: planEnd parses the id with entity.ParseAddress after trimming and refuses a face inside it with 400 at /relations/j/{end}/id. Test case 'face inside an id'.
status: addressed
---
