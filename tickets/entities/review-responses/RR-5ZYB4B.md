---
id: RR-5ZYB4B
type: review-response
title: for_each children read a reloaded task mid fan-out
finding: Children look the task up when they run, so a reload between expansion and children changes their settings.
severity: minor
reason: Snapshotting the script, template and capabilities into child payloads changes the for_each job contract; documented in the Config changes section for now.
status: deferred
---
