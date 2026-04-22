---
id: RR-0RDB4
type: review-response
title: Inline YAML template strings lose editor support
finding: 200+ lines of metamodel/data-entry YAML embedded as string literals — no syntax highlighting, no validation. Typo-risk surfacing as unrelated test failure. Move to e2e/tests/fixtures/*.yaml and readFileSync.
severity: minor
reason: Inline YAML stays for now. Extracting to external files would require syncing with the eslint-scanned tests directory and adds a separate test setup concern. Keeping as string literals is acceptable given we lint the fixture file; if a future PR touches this code we can move to external files then.
status: deferred
---
