---
id: RR-5XC3XJ
type: review-response
title: NeedsCeilingCheck false for permission sources
finding: grantsPermission consults only permitsPermission, not the ceiling's read/update/deny_write axes; a deny_write agent baseline still posts via a permission role; docs promise otherwise
severity: significant
resolution: NeedsCeilingCheck takes a Capability and is false only for attached on read; docs list what each ceiling axis narrows.
status: addressed
---
