---
id: BUG-AVHQA6
type: bug
title: CLI rename runs without a principal
description: rela rename passes context.Background() so the write has no principal and no audit author.
priority: low
effort: s
status: backlog
---

## Description

`rela rename` passes `context.Background()` (internal/cli/rename.go:194), so the
write carries no principal. The policy ACL treats it as unstamped and the audit
log has no author.

## Fix

Use the command context with the CLI principal, as the other CLI write commands
do.

Found during TKT-NXRFDU.
