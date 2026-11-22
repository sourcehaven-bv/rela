---
id: RR-9ZJKYHL
type: review-response
title: "The tests set the flag by hand and miss the shortcut"
finding: "The read-only test set workspaceReadOnly directly, the dry-run failed with a plain Error, the Cmd+Enter path had no test, and the field selector matched any input."
severity: significant
resolution: "Addressed: the form test serves the dry-run through the real API client with an axios 403 tenant_read_only, asserts #field-title, and checks that neither the shortcut nor typing sends a POST. Each fix was reverted once to confirm a test fails."
status: addressed
---
