---
id: RR-CVD9V5
type: review-response
title: Guard errors name hidden owned entity ids
finding: channelguard.go list/detach errors include the owned entity id; MCP and Lua return err.Error() to the caller, bypassing the ownedDeleteError rule that names only the owner.
severity: minor
status: open
---
