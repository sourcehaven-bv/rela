---
id: AM-exec-commands-set-env
type: automated-measure
title: Every exec.Command sets Env from cmdexec.Environ
description: internal/archguard TestExecCommandsSetEnv scans every non-test Go file under internal/ and cmd/ and fails on an exec.Command or exec.CommandContext call in a function that never assigns Env. Without Env a child inherits RELA_TOKEN_KEY, RELA_DATABASE_URL and every REMOTE_CREDENTIAL_ variable. The allowlist is empty.
kind: test
location: internal/archguard/execenv_test.go
status: active
---

Added for BUG-BU16GBM (GH #2242). Verified by removing one Env assignment: the
test failed naming the file and line.
