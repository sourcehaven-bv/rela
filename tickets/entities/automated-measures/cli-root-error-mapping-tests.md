---
id: cli-root-error-mapping-tests
type: automated-measure
title: Test wrapDiscoverError preserves underlying errors
description: Unit test verifying that wrapDiscoverError returns the 'run rela init' hint only for errors.ErrNoProject, and surfaces all other errors (e.g. metamodel parse failures) verbatim.
kind: test
location: internal/cli/root_test.go:TestWrapDiscoverError
status: active
---
