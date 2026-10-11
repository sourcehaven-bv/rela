---
id: RR-O4Y9UA
type: review-response
title: t.Fatalf called from non-test goroutines
finding: 'Cranky review: retitle called t.Fatalf inside goroutines in AC2, AC3 and AC9.'
severity: minor
resolution: Added later(), which writes on a goroutine and reports with t.Errorf; retitle stays on the test goroutine only.
status: addressed
---
