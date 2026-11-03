---
id: RR-UP4K1H
type: review-response
title: Test helper reimplements strconv.Itoa
finding: sweep_test.go:532 itoa duplicates strconv.Itoa; use the standard library.
severity: nit
resolution: The helper is removed and the tests use strconv.Itoa.
status: addressed
---
