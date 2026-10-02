---
id: RR-4C7NF5
type: review-response
title: Tests miss the risky cell shapes
finding: No case for a multi-value enum, non-enum list, boolean, relation column or server-format date; the date assertion was too weak.
severity: significant
resolution: EntityDetail.table.test.ts covers title, enum and list enum, non-enum list, date, boolean true and false, undeclared list, relation column and empty cells; fixtures carry propType like the server. Title, list and date tests fail against develop.
status: addressed
---
