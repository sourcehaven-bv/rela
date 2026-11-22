---
id: RR-VIEVNFK
type: review-response
title: "An empty read-only cell crashes the section"
finding: "isSectionCellEmpty called values.every, but the server sends null values for an unset property or a relation without targets."
severity: critical
resolution: "Addressed: values ?? [] guards null; sectionTable.test.ts covers a null values cell."
status: addressed
---
