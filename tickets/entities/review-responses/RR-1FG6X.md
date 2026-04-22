---
id: RR-1FG6X
type: review-response
title: 'Add config-load-time existence check for script: path'
finding: Existing CheckActionScriptExists pattern (action.go:105) fails fast at server startup when a referenced script is missing. Plan only handles missing-script at render time, which is a deferred HTTP 500 instead of clear startup error.
severity: minor
resolution: Config-load existence check mirroring CheckActionScriptExists for document scripts. Plan approach §1.
status: addressed
---

From design-review on PLAN-78HJO. Low effort; mirror the action-script existence
check in the document validator.
