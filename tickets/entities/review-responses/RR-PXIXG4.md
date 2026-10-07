---
id: RR-PXIXG4
type: review-response
title: Document renders and webhooks get tag writes
finding: App.luaWriteDeps sets VersionTags for action scripts, document renders, export_render and the webhook path; deps.go comment says renders leave it nil.
severity: minor
resolution: Tag writer reaches action scripts only; TestLuaWiring_VersionTagsReachActionsOnly.
status: addressed
---
