---
id: RR-TGR0P8
type: review-response
title: Wire drift check does not touch the TypeScript interfaces
finding: 'frontend/src/types/channels.ts CHANNEL_WIRE_FIELDS is a free-standing Record<string, readonly string[]>. wireFields.test.ts compares Go''s field set with that string list, not with the interfaces (Channel, Message, ...). Two failure modes: rename `edited_at` in the Go struct, the golden and the list, and forget the interface, and everything is green while the SPA reads undefined. Or edit the interface and forget the list, and nothing fails. AC 20 claims ''wire field sets match'', but only the list is checked. Fix: tie each list to its interface at the type level, for example `const ChannelKeys = [...] as const satisfies readonly (keyof Channel)[]` plus an exhaustiveness assertion `type _ = Assert<Exclude<keyof Channel, (typeof ChannelKeys)[number]> extends never ? true : false>`, so vue-tsc fails on drift in either direction. Then build CHANNEL_WIRE_FIELDS from those consts.'
severity: significant
resolution: CHANNEL_WIRE_FIELDS is built with a wireKeys helper that fails vue-tsc when a key list and its TypeScript interface drift in either direction, so the Go golden now reaches the interfaces.
status: addressed
---
