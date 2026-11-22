---
id: RR-7Y4G7TF
type: review-response
title: "An editor sees only the own preferences when /_me fails"
finding: "SettingsView gates the shared cards on /_me; if that request fails, a settings editor sees only the own preferences."
severity: minor
reason: "Without /_me the app shell is degraded anyway, and a reload fixes it. Showing the shared cards on a failure would bring back the refused controls for everyone else."
status: wont-fix
---
