---
id: RR-FS4Z60
type: review-response
title: Page-size test helper is a process global
finding: SetIteratorPageSizeForTest changes a package global that parallel tests share.
severity: nit
reason: The value is atomic, restored on cleanup, and the tests using it do not run in parallel; any page size is correct, so a concurrent reader only sees a different page size.
status: wont-fix
---
