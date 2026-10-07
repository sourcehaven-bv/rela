---
id: RR-2GAESG
type: review-response
title: Tables is exported only for a test
finding: appbuild SQLiteData.Tables exists for the cli table-accounting test.
severity: nit
reason: The test lives in package cli and needs the table list; the method is small and read-only.
status: wont-fix
---
