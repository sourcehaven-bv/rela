---
id: RR-6SX6HC8
type: review-response
title: 'Review: CLI and db paths do not re-check after the lock'
finding: openProject and resolveDSN pin the schema after reading the status and only the 30s watcher catches a change in between; db commands had no watcher.
severity: minor
resolution: WatchDeployment resolves once immediately after the pin; resolveDSN now also runs the watcher.
status: addressed
---
