---
id: RR-CCNILQ
type: review-response
title: Channel API paths interpolate address unencoded
finding: 'frontend/src/api/channels.ts:43 builds /{plural}/${a.address}/_channels without encodeURIComponent, and live.ts liveChannelsPath/liveChannelPath reuse it (channelType and name are encoded, address is not). If a future consumer passes an address taken from a route param or other attacker-influenced input, a value such as ../_piles/x?y= makes the SPA send GET/POST/PATCH/DELETE to a different /api/v1 path with the user''s session (client-side path traversal). Unverified: no component consumes useEntityChannels/useChannel yet, and the same pattern pre-exists in api/presence.ts:16. Fix: validate address against the REF_ID shape already defined in utils/markdown.ts (^[A-Za-z0-9][A-Za-z0-9_.-]*(@[A-Za-z0-9][A-Za-z0-9_.-]*)?$) in channelsPath and throw on mismatch, or encode the id and face parts separately; apply the same to presence.ts.'
severity: minor
resolution: channelsPath and the presence paths validate the address with a shared ENTITY_ADDRESS check that throws on anything that is not id or id@face, with tests.
status: addressed
---
