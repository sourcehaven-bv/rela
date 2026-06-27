---
id: api-error-boundary-contract-test
type: automated-measure
title: 'Contract tests: API client rejection shape (ApiError) for all failure classes'
description: 'Vitest suite (frontend/src/api/errors.test.ts) pinning what a catch site receives from the shared client for each failure class: ProblemDetail responses, script_error envelopes, cancellations (ERR_CANCELED/ECONNABORTED), missing responses, and unstructured error bodies — plus getErrorMessage/getScriptError extraction tables. Prevents the boundary contract from drifting unpinned again (BUG-X9VNE1 why4).'
kind: test
location: frontend/src/api/errors.test.ts
status: active
---
