---
id: RR-YLUPO3O
type: review-response
title: Command runner not reachable from realtime wiring
finding: The scan/transform runner is built in dataentry.NewApp; realtime.Deps has none; the CLI uses a nil runner.
severity: significant
resolution: Runner is built in appbuild and passed to both dataentry and realtime. With no runner a configured scan refuses every file (fail closed); CLI tests do not configure a scan.
status: addressed
---
