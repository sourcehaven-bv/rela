---
id: AM-lua-tenant-heap-budget
type: automated-measure
title: Lua script heap growth is capped per runtime
description: A test runs a script that grows memory through string concatenation, gsub and table inserts, and asserts the runtime aborts once its heap budget is crossed while other runtimes keep working.
kind: test
location: internal/lua (planned test with BUG-88F0XLW)
status: proposed
---
