---
id: RR-6P8QL8
type: review-response
title: If-Match compared against a cached ETag that only CalDAV writes refreshed
finding: checkPreconditions compared the client's If-Match against Alias.ETag, which is refreshed only by CalDAV writes. Any rela-side edit (SPA, CLI, MCP, automation, git pull) left it stale, so the stored and served tags drifted. Both directions lost data. (1) The client presents the tag the server JUST served and gets 412 - permanently, since only a successful CalDAV write refreshes the stored tag, which is what is being refused; the client wedges. (2) A client holding the STALE tag succeeds and silently overwrites the newer rela-side edit - the exact overwrite If-Match exists to prevent. The doc comment claimed the opposite of the actual behaviour.
severity: critical
resolution: Compare against the ETag the resource would be served right now, re-rendered via caldavBackend.currentETag (ACL-scoped through feedEntitySource.getEntity). Removed the Alias.ETag field entirely so no second place exists for a conditional-request value to drift, with a doc note on Alias explaining why it must not come back. Regression test TestCalDAV_IfMatchComparesAgainstCurrentContent asserts both directions; verified to fail against a simulated drifted tag. Both directions re-verified live against the demo (fresh -> 201, stale -> 412).
status: addressed
---
