---
id: RR-8YLH7U
type: review-response
title: CheckOwningEdge no longer needs export
finding: Its only external caller was the removed fallback.
severity: minor
resolution: ""
reason: owning_restore_test.go (package entitymanager_test) calls it, so it stays exported.
status: wont-fix
---
