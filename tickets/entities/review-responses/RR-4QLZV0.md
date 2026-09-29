---
id: RR-4QLZV0
type: review-response
title: Only the first bad literal is reported
finding: A property with several mistyped literals needs one reload per literal.
severity: minor
resolution: checkEnumLiterals collects every bad literal into one message; test 'all bad literals reported'.
status: addressed
---
