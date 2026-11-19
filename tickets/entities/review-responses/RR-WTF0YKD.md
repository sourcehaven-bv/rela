---
id: RR-WTF0YKD
type: review-response
title: Fixture grants alice the auditor role for every job test
finding: The ACL change in buildJobProject applies to every job test.
severity: nit
reason: No job test uses alice as a principal; the grant only adds a reader.
status: wont-fix
---
