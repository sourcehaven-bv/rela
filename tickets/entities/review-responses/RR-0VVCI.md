---
id: RR-0VVCI
type: review-response
title: Test files use context.Background() instead of t.Context()
finding: Some _test.go files (api_v1_test.go, app_test.go, document_script_test.go, views_test.go, helpers_test.go) use context.Background() for the newly-added ctx args where the package elsewhere uses t.Context(). Stylistically inconsistent within the same file.
severity: nit
reason: Acceptable per ticket scope (test-file Background() explicitly allowed in the planning AC). Several call sites are in helpers with no *testing.T in scope (e.g. implementsTargets(app, entityID)), where Background() is the only option. A churn-only follow-up to standardize on t.Context() is out of scope for this lint-enablement ticket.
status: wont-fix
---
