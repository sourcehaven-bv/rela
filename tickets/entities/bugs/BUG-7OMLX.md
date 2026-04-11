---
id: BUG-7OMLX
type: bug
title: Scheduler uses unexported workspace.Meta() after Snapshot refactor
why1: workspace.Meta() was made unexported to meta() in the Snapshot refactor (#372)
why2: The scheduler PR was developed against the pre-refactor API
why3: Squash merge resolved the conflict textually but not semantically
prevention: CI catches the build failure; this fix resolves it
status: done
severity: high
---
