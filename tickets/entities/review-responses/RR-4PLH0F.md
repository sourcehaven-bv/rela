---
id: RR-4PLH0F
type: review-response
title: Naive graph query test accepts empty-body note rows
finding: The '|| e.Type == "note"' clause in the assertion at graphquery_naive_test.go:86 lets a note row with an empty body pass 'rows keep their body'.
severity: nit
reason: This behavior predates the commit and is outside the scope of BUG-RNM0MU, which only changes the assertion form.
status: wont-fix
---
