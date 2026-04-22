---
id: RR-0RDB4
type: review-response
title: Inline YAML template strings lose editor support
finding: 200+ lines of metamodel/data-entry YAML embedded as string literals — no syntax highlighting, no validation. Typo-risk surfacing as unrelated test failure. Move to e2e/tests/fixtures/*.yaml and readFileSync.
severity: minor
status: open
---
