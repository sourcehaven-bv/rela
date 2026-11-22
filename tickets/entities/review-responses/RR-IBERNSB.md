---
id: RR-IBERNSB
type: review-response
title: usePageTitle leaks outside a scope
finding: Without a scope onScopeDispose never removes the entry.
severity: minor
resolution: usePageTitle throws without a current scope; a test pins it.
status: addressed
---
