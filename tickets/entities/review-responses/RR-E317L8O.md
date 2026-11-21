---
id: RR-E317L8O
type: review-response
title: The composable hides a side effect inside an argument
finding: useDocumentTitle(useShellAppConfig(schemaStore)) reads as a pure value but also installs the language watcher.
severity: minor
resolution: It now returns { appName } and App.vue calls it on its own line before useDocumentTitle, with a comment that it also feeds app.language.
status: addressed
---
