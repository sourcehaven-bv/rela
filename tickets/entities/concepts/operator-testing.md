---
id: operator-testing
type: concept
title: Operator Testing
summary: Automated tests that operators write for their own rela app config (schema, ACL, automations, scripts, schedules)
description: Tests that operators write for their own rela app config. rela test runs them in an isolated copy of the project with a fake clock and captured side effects. Not to be confused with test-fixtures and test-isolation, which cover rela's own Go tests.
package: internal/apptest
layer: cli
status: draft
---

## Description

Operator testing lets the people who configure a rela project check that the
config does what they intend. Tests are `.test.lua` files in the project's
`tests/` folder. `rela test` runs them against fixtures in a throwaway store,
with a fake clock and fake side effects (mail, HTTP, AI, jobs). The same suite
run on a newer rela binary shows whether an upgrade changes the app's behaviour.

Distinct from `test-fixtures` and `test-isolation`, which are about rela's own
Go test suite.
