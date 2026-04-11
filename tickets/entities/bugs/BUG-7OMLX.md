---
id: BUG-7OMLX
type: bug
title: Scheduler uses unexported workspace.Meta() after Snapshot refactor
why1: workspace.Meta() was made unexported to meta() in the Snapshot refactor (#372)
why2: The scheduler PR was developed against the pre-refactor API
why3: Squash merge resolved the conflict textually but not semantically
why4: No integration test verifies the scheduler compiles against the real workspace type
why5: Interface-based decoupling hides compile errors until the caller wires them together
prevention: CI catches the build failure; this fix resolves it
description: Scheduler WorkspaceProvider interface declared Meta() but workspace.Workspace only exports meta() after the Snapshot refactor
status: done
severity: high
---
