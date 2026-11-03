---
id: RR-QE82UD
type: review-response
title: Channel reads drop the page world, contrary to R14 and the api contract
finding: 'OpenChannel builds its target without `world` (OpenChannel.vue:80-85) and ChannelsSection builds the anchor without it (ChannelsSection.vue:39), so listChannels, getChannelHead, getMessages and liveChannelPath never carry `?world=`. Plan R14 says every channel URL and the live path are built from the entity ref plus the page `?world=`; api/channels.ts:21-23 says world goes on reads; livepath.go:57-60 says both channel routes resolve the anchor in a world. The ChannelsSection doc comment (lines 14-15, ''Channel routes take no ?world='') and the OpenChannel prop doc (line 65) assert the opposite, with no recorded decision. On a faceless type the anchor and ref resolution then run in the default world while the page is in another. Fix: pass `world: props.world` in both the anchor and the target (api/channels.ts already keeps it off writes), correct the two comments, and add a ChannelsPanel test asserting that the list, head, page and live path carry the world. If dropping the world is intended, record that in the plan and fix api/channels.ts and livepath.go to match.'
severity: significant
resolution: Verified in Go that attachWorld answers a non-default world on every _channels and _presence route with 422 world_unsupported, so the SPA now sends no world on any channel read or live path (world removed from ChannelAnchor), and the comments in api/channels.ts, ChannelsSection, OpenChannel and livepath.go plus plan R14 state that, with a test asserting no channel request or live path carries a world.
status: addressed
---
