---
id: RR-2WA7R
type: review-response
title: State file writes must use atomic write pattern
finding: The plan mentions writing .rela/scheduler-state.json but doesn't specify write safety. The codebase already has internal/storage/safefs.go with atomic write-to-temp+fsync+rename pattern, exposed via repository.WriteCacheFile(). A crash mid-write without atomic writes would corrupt the state file, causing all tasks to re-execute on next startup.
severity: significant
resolution: Updated plan to specify atomic writes via internal/storage/safefs.go pattern
status: addressed
---
