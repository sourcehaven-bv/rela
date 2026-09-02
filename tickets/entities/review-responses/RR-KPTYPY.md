---
id: RR-KPTYPY
type: review-response
title: 'executeScriptActions stamps a dangling automation: for a nameless automation'
finding: The scripted path is unguarded twelve lines from the helper whose doc says a dangling colon is worse than the generic label
severity: significant
resolution: Fixed by routing runner.go's scripted path through triggeredByCtx - but only AFTER the helper itself was made to compose (RR-ZYVERL); landing it against the clobbering version would have spread that bug to a fourth call site. Now all three cascade write paths share one guarded helper. Pinned by TestRunnerScriptActionTriggeredByLabel (table test covering named and nameless).
status: addressed
---

`internal/autocascade/runner.go:298` (the pre-existing scripted path) does:

```go
actionCtx := audit.WithTriggeredBy(ctx, "automation:"+action.AutomationName)
```

Unguarded. For an automation with no `name:`, that writes the literal string
`"automation:"` into the audit log — exactly what this ticket's new
`triggeredByCtx` helper exists to avoid, and its doc says so verbatim: *"a
dangling colon would be worse than the generic label it replaced."*

`internal/script/luascriptrunner.go:213-227` guards the same concept correctly
(`action.FilePath == "" && action.Name != ""`), so the tree now holds three
different answers to one question across two packages.

Predates this change, but the implementation checklist claimed the tag was *"a
single triggeredByCtx helper shared by the relation and entity paths"* — it
should be shared with the script path too. Routing line 298 through the helper
is a one-line fix.

Second effect: the unguarded stamp also OVERWRITES a caller's existing label.
`scheduler.go:212` sets `schedule:<task-name>`; a scheduled task whose script
triggers a scripted automation action loses that attribution. Routing through
`triggeredByCtx` does not fix the overwrite by itself, but the empty-name case
is the half this ticket owns.
