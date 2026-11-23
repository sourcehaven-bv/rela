---
id: RR-KI680EG
type: review-response
title: "Retry never left the boot error screen"
finding: "The Retry button reloaded the schema but never cleared the error, so the error screen stayed after a successful reload, and a failed reload was an unhandled rejection."
severity: minor
resolution: "Retry reads the status again and then runs the same load as the start, which clears the error, shows the loading screen and catches a failure."
status: addressed
---
