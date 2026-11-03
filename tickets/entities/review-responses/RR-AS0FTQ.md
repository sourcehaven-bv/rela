---
id: RR-AS0FTQ
type: review-response
title: Exported type names shadow the app wire types
finding: index.ts exports `ChannelState` and `ChannelPropertyType` via `export type *`, and the same names exist in frontend/src/types/channels.ts. ChannelsPanel (T7c) imports both and must alias one. Consider prefixing the library ones (for example `ChannelStateView`), or drop the library duplicates and reference the structural types only through the Def interfaces.
severity: nit
resolution: The library types were renamed to ChannelStateView and ChannelPropertyKind so they no longer shadow the app wire types.
status: addressed
---
