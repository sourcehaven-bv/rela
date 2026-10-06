---
id: DEC-325POW
type: decision
title: The Configure save path may write and run data migrations from rela-server
context: CLAUDE.md says Gate.Persist is called only by the CLI, because a server writing a git-tracked file at boot would dirty a working tree and race concurrent starts. In-app configuration editing (FEAT-MPH34X) writes schema.yaml itself, so a change that needs migrating must produce and apply its migration in the same request or the server would serve a schema its data does not fit. The user chose 'server may persist'.
consequences: 'Only the Configure save path does this: an explicit operator action by a principal holding config:edit, never at boot. It runs under the save mutex, lets the runner take the migration lock (ErrLockHeld becomes 409), never persists drift, rolls back only before the runner starts and rolls forward after. Server boot still never calls Persist. The fs tier commits applied.json as before; the server dirties the working tree only on an explicit save, as any other write does. CLAUDE.md''s data-migration section gets this exception.'
date: "2026-10-05"
status: accepted
---

## Decision

The Configure save path (TKT-F5NGMG) may write a data migration file, run it
through the shared `datamigration` runner and record it in the per-store state,
from inside `rela-server`. `Gate.Persist` and `Runner.Run` stop being CLI-only
for this one caller.
