---
id: RR-O5V09FC
type: review-response
title: A foreign add_to_pile can fill and evict another user's declared pile
finding: Push routed a plain pile name equal to a declared key to pushDeclared, which skips the foreign-push bounds (KeepExisting, Create, MaxForeignPiles). Any user could push 500 refs to owner=victim pile=favorites through MCP or Lua and evict every favorite.
severity: critical
resolution: Push now drops a foreign push to a declared pile unless it comes by Key (only the notify cascade sets Key) onto a pile with read state. TestService_ForeignPushReachesOnlyNotificationPiles pins favorites by key and by name and inbox by name. Documented in the data-entry guide's Privacy section.
status: addressed
---
