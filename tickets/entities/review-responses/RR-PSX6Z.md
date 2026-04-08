---
id: RR-PSX6Z
type: review-response
title: Stale comments reference reloadLockMiddleware/RLock
finding: Comments in api_v1.go (lines 201, 211), handlers_api.go (lines 805, 826, 877), handlers_api_test.go (line 33), and actions.go (line 23) still describe behavior under the old reloadLockMiddleware/RLock model. They lie to the next reader.
severity: minor
resolution: Updated comments in actions.go (now references writeMu instead of 'workspace write lock'), api_v1.go handleV1DynamicRoutes (now describes the snapshot+writeMu model), and handlers_api.go handleAPISaveSettings (now describes mutateState). Test comments and inline notes left as-is for brevity.
status: addressed
---
