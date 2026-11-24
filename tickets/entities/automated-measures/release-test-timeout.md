---
id: release-test-timeout
type: automated-measure
title: 'Workflow: release tests run with a 20 minute package timeout'
description: The Release workflow runs the suite with the same package timeout as CI.
kind: workflow
location: .github/workflows/release.yml
status: active
---

The Test job passes -timeout 20m to go test.
