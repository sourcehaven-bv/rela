---
id: RR-QCFVG6
type: review-response
title: Data-entry watcher would broadcast a false config error
finding: The old App's reloadConfig validates the new data-entry.yaml against its old metamodel (watcher.go:350-356).
severity: significant
resolution: 'Plan changed: the save stops the old App''s config watch before writing (restarted on restore); the save mutex lives in one configedit.Service built in main and mounted into each App via a setter.'
status: addressed
---
