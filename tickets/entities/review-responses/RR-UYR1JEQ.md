---
id: RR-UYR1JEQ
type: review-response
title: "Board search placeholder has no unit test"
finding: "KanbanView builds the same placeholder as the list without a test."
severity: minor
reason: "It is the same two-line computation over schemaStore.pluralLabel, which EntityList.empty.test.ts covers; a board mount harness for one string adds little."
status: wont-fix
---
