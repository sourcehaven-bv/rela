---
id: RR-8FB1NHF
type: review-response
title: Differential test only runs on memstore
finding: Emptiness, != on a missing value and enum ranks were only proven against graphquerynaive.
severity: minor
resolution: 'Addressed: searchplan_postgres_test.go runs both differential tests on pgstore. It found that pgstore GraphPosition ignored OrderSpec.Values; position.go now orders its window with orderKeySQL, and storetest RunGraphPositionTests gains ranked cases that fail without the fix.'
status: addressed
---
