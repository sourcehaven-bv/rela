---
id: RR-VEEAZO
type: review-response
title: Reconcile does not gate on scope compile errors
finding: scopeTraversalSpecs skips a broken scope with a warning, so reconcile would drop its index.
severity: nit
reason: The server and every CLI entry point refuse to load a schema whose scopes do not compile (scopes.Compile in prepare), so reconcile never runs against one.
status: wont-fix
---
