---
id: RR-YPXVWQ
type: review-response
title: Capacity and owning checks run twice
finding: They run before the entity write and again inside writeRelationCreate.
severity: minor
reason: The second check is the standard check in writeRelationCreate, which every relation write runs. It costs one query per conversion, and removing it would add a bypass to the shared path.
status: wont-fix
---
