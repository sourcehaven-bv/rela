---
id: check-flag-shorthands
type: automated-measure
title: Check flag shorthand availability before adding
kind: test
location: internal/cli/root_test.go:TestNoShorthandConflicts
status: active
description: Test that walks all CLI commands and verifies no local flag shorthand conflicts with a persistent flag shorthand from a parent command. Runs in CI on every PR.
---
