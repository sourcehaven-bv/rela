---
id: RR-JGCXRFL
type: review-response
title: listRowsFor claimed a query shape it did not enforce
finding: The doc said query must hold only the list filters and a sort, but nothing enforced it; tableSort also built a whole query to read its sort.
severity: nit
resolution: listRowsFor now takes the sort and the reach as parameters and builds the query itself.
status: addressed
---
