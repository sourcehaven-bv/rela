---
id: RR-59QA3E
type: review-response
title: Guard logic lives in appbuild
finding: About 330 lines of guard, reconciler and record logic sit in internal/appbuild/channelguard.go; the plan placed it in internal/realtime.
severity: nit
reason: The guard is still in internal/appbuild/channelguard.go (277 lines) with the record check in realtime_service.go. It implements entitymanager.AnchorGuard and reads the appbuild RealtimeConfig holder; internal/realtime has no entitymanager dependency, so the move needs a new arch-lint edge and a narrowed config accessor. Moving it is a refactor with no behaviour change and is left for a follow-up.
status: deferred
---
