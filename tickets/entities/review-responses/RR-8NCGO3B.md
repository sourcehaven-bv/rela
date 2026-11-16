---
id: RR-8NCGO3B
type: review-response
title: GOFLAGS=-race also reaches go subprocesses started by tests
finding: On push, a test that runs go build or go run would inherit -race.
severity: nit
reason: No test does this today; the reviewer found none.
status: wont-fix
---
