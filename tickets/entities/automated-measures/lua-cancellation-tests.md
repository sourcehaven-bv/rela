---
id: lua-cancellation-tests
type: automated-measure
title: Lua runtime cancellation tests
description: Tests that verify a parent context.Context cancellation propagates into the gopher-lua LState and interrupts execution well before the internal timeout fires. Covers WithContext option and busy-loop interruption.
kind: test
location: internal/lua/runtime_test.go
status: active
---
