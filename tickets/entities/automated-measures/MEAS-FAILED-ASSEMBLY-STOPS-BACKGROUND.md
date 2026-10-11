---
id: MEAS-FAILED-ASSEMBLY-STOPS-BACKGROUND
type: automated-measure
title: A failed assembly stops every background service it started
description: Test that forces finishAssembly to fail and asserts no mail, GC or soft-delete goroutine survives.
kind: test
location: internal/appbuild/appbuild_test.go
status: proposed
---
