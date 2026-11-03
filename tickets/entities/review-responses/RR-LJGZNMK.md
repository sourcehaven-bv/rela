---
id: RR-LJGZNMK
type: review-response
title: Tests miss header edge cases and a next sentinel
finding: Only the no-header case was tested. The 500 test used NotFoundHandler instead of a called sentinel.
severity: minor
resolution: 'The 401 test is now a table: no header / placeholder value unknown / header sanitized to empty. The 500 test asserts next is not called.'
status: addressed
---
