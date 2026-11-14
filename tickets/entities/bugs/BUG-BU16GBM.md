---
id: BUG-BU16GBM
type: bug
title: Fixed helper commands inherit REMOTE_CREDENTIAL_ variables
description: 'Four fixed helper commands (git config, the bwrap and sandbox-exec probes, cp -r in rela-docs) set no Env and inherited the full process environment, so the documented guarantee that no child inherits a REMOTE_CREDENTIAL_ variable was false (GH #2242).'
priority: low
effort: xs
why1: Four helper commands (git config, the bwrap and sandbox-exec probes, cp -r in rela-docs) set no Env, so Go gave them the full process environment.
why2: Setting Env was a per-call-site choice; only configured commands (cmdexec.Runner, rela graph, data-entry commands) were routed through cmdexec.Environ.
why3: The fixed helpers were treated as harmless because they run no operator content, so no one applied the withholding rule to them.
why4: The godoc and the security guide stated the rule for every child, but nothing checked that every exec.Command call follows it.
why5: A security invariant over all process spawns was enforced by convention rather than by a tree-wide test.
prevention: internal/archguard TestExecCommandsSetEnv fails on any exec.Command or exec.CommandContext call in a function that never assigns Env; its allowlist is empty.
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

## Symptom

GitHub issue #2242. The ACL security guide says no external command rela starts
inherits a `REMOTE_CREDENTIAL_` variable, and the godoc of `cmdexec.Environ`
says every command rela starts gets its environment from there. Four fixed
helper commands set no `Env` and inherited the full process environment:

- `internal/automation/template.go`: `git config user.name` / `user.email`
- `internal/cmdexec/sandbox_linux.go`: the `bwrap` sandbox probe
- `internal/cmdexec/sandbox_darwin.go`: the `sandbox-exec` sandbox probe
- `internal/docscapture/server.go`: `cp -r` in the `rela-docs` tool

The leak risk was low: these are fixed binaries with no operator content.

## Fix

Each of the four sets `Env` from `cmdexec.Environ()`. A new archguard test,
`TestExecCommandsSetEnv`, fails on any `exec.Command` or `exec.CommandContext`
call in a function that never assigns `Env`. Its allowlist is empty.
