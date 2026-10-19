---
id: RR-II9BO2
type: review-response
title: ReloadDeps should refuse a bundle without Channels.Service once channel tools are registered
finding: Tools are registered once at NewServer, but validateFor knows nothing about channels. A reload whose deps carry a nil Service, for example from NewFromCollaborators, would nil-panic in channelHandler. Add a check alongside the luaTools one.
severity: minor
resolution: validateFor refuses a reload bundle without Channels.Service once channel tools are registered.
status: addressed
---
