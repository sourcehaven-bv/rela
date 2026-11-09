---
id: FEAT-IODR9ST
type: feature
title: 'Operator test framework: rela test'
summary: Operators write .test.lua tests for their app config and run them in CI and before upgrading rela
description: Operators write .test.lua files that run against fixtures in a throwaway copy of their project. Tests act as principals through the real write path and assert on entities, relations, mail and access. A recorded trace later enables coverage, impact analysis, mutation testing, property-based tests and comparison across rela versions.
priority: medium
status: proposed
---

## Description

`rela test` runs `.test.lua` files against fixtures in a throwaway store. Tests
act as principals through the real write path, control a fake clock, run
schedules, drain jobs and assert on entities, relations, mail and outgoing HTTP.

The harness records a per-test trace (automations fired, ACL grants that
decided, transitions, validations, side effects). Later slices build on the
trace:

- config coverage and impact analysis
- config mutation testing
- self-explaining failures and executable documentation
- record mode from the data-entry UI
- property-based tests with built-in properties and shrinking to `.test.lua`
- differential trace comparison across rela versions, plus `migrate --check`
