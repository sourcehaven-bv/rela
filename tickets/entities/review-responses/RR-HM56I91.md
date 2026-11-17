---
id: RR-HM56I91
type: review-response
title: Create outputs could exceed max and orphan entities
finding: A create proposal was not bounded by max_outgoing less existing relations, and one relation could appear in both relate and create.
severity: critical
resolution: llmtask.createRoom bounds create maxItems by cardinality less Input.CreateExisting; config refuses a relation in both lists; a failed relation deletes the new entity.
status: addressed
---
