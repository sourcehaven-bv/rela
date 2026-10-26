---
id: RR-9S9L8Z
type: review-response
title: Scope is xl; split it
finding: 'The slice touches realtimecfg, acl, channels and three backends, realtime (service, runner, timers, msgops), entitymanager, appbuild, apiwire, dataentry, openapi, mcp, cli, the frontend types and the docs, with 19 ACs. Plan change: split it. 5a: message-driven outputs (config, templates, the mark on Append, inline run, set and create, the pre-check, wire). 5b: timer and trigger outputs with run_as, the grant check, the runner with claims and retries, and on_final close. 5b adds only SetSession marks and the runner on top of 5a''s mark format.'
severity: minor
resolution: Adopted the coordinator's split. 5a covers message-driven outputs, the pre-check, marks with runner retries, on_final and wire/docs. 5b covers run_as, the grant check, and timer and trigger outputs.
status: addressed
---
