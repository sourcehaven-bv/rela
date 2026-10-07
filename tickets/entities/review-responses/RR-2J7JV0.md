---
id: RR-2J7JV0
type: review-response
title: WAL check treats any stat error as left behind
finding: Finish reported EACCES as a leftover WAL file.
severity: nit
resolution: Only a successful stat fails as left behind; other errors are returned as they are.
status: addressed
---
