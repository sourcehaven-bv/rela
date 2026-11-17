---
id: RR-L436LH6
type: review-response
title: service.go not gofmt clean
finding: Field alignment in service.go was off after adding flows fields.
severity: significant
resolution: Ran gofmt -w; gofmt -l is clean.
status: addressed
---
