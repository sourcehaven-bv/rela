---
id: RR-FB98MQ
type: review-response
title: Search loads every candidate hit one at a time
finding: search.Service.Search called GetEntity per backend hit before the field check (index.go). Visible asks for candidates uncapped so a query matching 5000 rows made 5000 reads to serve 20 hits on fs, memstore and sqlite.
severity: significant
resolution: Service.Search now reads hit headers in pages of 256 with one ListEntityHeaders call per page (8bd4c30e9). No body is read. TestListBodies_QueryShapesKeepTheBound asserts zero bodies for narrow and broad ?q= at 10 and 50 rows; golden read counts updated.
status: addressed
---
