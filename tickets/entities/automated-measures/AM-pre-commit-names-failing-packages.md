---
id: AM-pre-commit-names-failing-packages
type: automated-measure
title: Pre-commit hook names the failing test packages
description: The hook prints the FAIL and panic lines of its go test run instead of discarding them, so a timeout in an unrelated package is visible and can be told apart from a real failure.
kind: ci
location: scripts/pre-commit (go test step)
status: proposed
---
