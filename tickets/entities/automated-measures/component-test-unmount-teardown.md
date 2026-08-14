---
id: component-test-unmount-teardown
type: automated-measure
title: Component tests unmount in afterEach so no component outlives its test
description: Ensures a mounted component cannot keep running async work after its test finishes. A leaked component's console.warn/error settles during vitest worker teardown and is reported as an unhandled error, failing CI with every test passing.
kind: test
location: frontend/src/components/forms/DynamicForm.test.ts (afterEach teardown in the shared mount helper)
status: proposed
---

Regression pin for BUG-2OXEW0.

The shared `mountEdit` helper registers every wrapper it creates, and an
`afterEach` unmounts them. Because all mounts funnel through that helper, no
individual test has to remember.

Why this is the measure rather than "retry the flaky job": the failure mode is
silent and misattributed. Every test passes, the exit code is 1, and the error
names a file the PR author never touched. Retrying hides it; unmounting removes
the race.

Applies beyond this one file — any component test that mounts something with
async lifecycle work has the same exposure. `DynamicForm` is simply the first
component with enough async surface (entity load, template load, dry-run
affordances, auto-link) for the race to become likely.
