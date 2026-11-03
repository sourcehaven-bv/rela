---
id: RR-MA9F2E
type: review-response
title: Watch replays reuse the call's cached ACL request
finding: 'Security and architecture design review: on remote MCP the watch call''s ctx carries the acl.Request that attachACLRequest opened. It caches global roles for its lifetime, so for up to 60 s replays would run on stale roles and a revoked caller could still get a changed bit. The plan wrongly claimed parity with slice 2, whose replays go through the full middleware chain.'
severity: significant
resolution: Each replay pass calls a new required Deps.Rebind (from appbuild.GatedReadBundle.Rebind), which detaches the attached request and binds a fresh one for the ctx principal. AC9 pins it by deleting a group membership mid-watch, with a mutation check.
status: addressed
---
