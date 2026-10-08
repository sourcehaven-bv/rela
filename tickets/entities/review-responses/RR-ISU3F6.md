---
id: RR-ISU3F6
type: review-response
title: ViewSection.relationOrder mixes camelCase and snake_case
finding: The section field is camelCase but RelationOrder's fields are snake_case.
severity: nit
reason: ViewSection fields are camelCase on the wire; RelationOrder is the list meta type, whose fields are snake_case like the rest of list meta. One type for both keeps one client type.
status: wont-fix
---
