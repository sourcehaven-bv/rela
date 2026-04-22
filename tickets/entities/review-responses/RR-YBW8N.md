---
id: RR-YBW8N
type: review-response
title: FormPage.submitAndExpectCreate dead assertion
finding: waitForResponse predicate already requires 201; subsequent expect(response.ok()).toBeTruthy() is dead. Replace with shape check.
severity: nit
reason: Nit. The redundant assertion is harmless; changing it risks rewriting for little value.
status: deferred
---
