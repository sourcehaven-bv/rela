---
id: RR-COLYCX
type: review-response
title: Watch names collide with the file watcher
finding: 'Architecture design review: Deps.Watcher and watcher.go are the file watcher; watch.go would add confusion.'
severity: nit
resolution: Files and type are tools_watch.go and watchHandler; the godoc says it is unrelated to the file watcher.
status: addressed
---
