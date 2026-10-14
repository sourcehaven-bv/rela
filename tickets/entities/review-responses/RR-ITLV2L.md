---
id: RR-ITLV2L
type: review-response
title: Non-string module names are accepted
finding: ls.CheckString converts numbers, so require(1) loads lib/1.lua; AC4 says non-string arguments are rejected.
severity: minor
resolution: require checks for an LString explicitly; numbers, tables and nil are argument errors. Covered in TestRequire_InvalidNames.
status: addressed
---
