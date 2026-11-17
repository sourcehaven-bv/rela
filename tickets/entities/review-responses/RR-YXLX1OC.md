---
id: RR-YXLX1OC
type: review-response
title: Action-script wiring was untested
finding: No dataentry test ran rela.append_records from an action.
severity: minor
resolution: Added TestActionScript_AppendsRecordsAsCaller and TestActionScript_NoAppendRecordsWithoutRecords; the test app helper now wires records like app.go.
status: addressed
---
