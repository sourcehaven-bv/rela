---
id: RR-0UW3B0
type: review-response
title: No CLI way to release a stuck action
finding: A promote that fails in a way the service cannot undo leaves the message action pending; ReleaseAction is the recovery and only HTTP offers it. A CLI or agent user would need the web app.
severity: significant
resolution: Plan adds channel action ID TYPE SEQ --release, calling Service.ReleaseAction (acceptance criterion 5).
status: addressed
---
