---
id: AM-remote-mcp-config-fail-closed
type: automated-measure
title: remote_mcp config refuses a null allow and a misspelled block next to a decode error
description: 'TestLoadConfig_RemoteMCP refuses an allow: with no value and a misspelled remote-mcp key in a file that also has a duplicate key elsewhere. Catches a config check that fails open (BUG for issue 2324).'
kind: test
location: internal/project/config_test.go (TestLoadConfig_RemoteMCP)
status: active
---
