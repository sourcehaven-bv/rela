---
id: RR-9KH2
type: review-response
title: TestHeaderPrincipalResolver_WeirdHeaderName proves the wrong thing
finding: 'Test sends X-User: alice but configures the resolver with X-Weird Name. It really tests "configured name doesn''t match sent name" rather than "weird name handled safely."'
severity: minor
reason: Test's intent (regression guard against r.Header.Get panicking on syntactically-invalid name) IS satisfied — it invokes r.Header.Get("X-Weird Name") and observes no panic. Whether the sent header also has weird chars is separate; Go's http.Header.Set rejects them at the source. The current test exercises the resolver path the operator actually controls. Rename to clarify intent is a follow-up cleanup; not blocking this PR.
status: wont-fix
---
