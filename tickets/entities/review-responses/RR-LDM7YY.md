---
id: RR-LDM7YY
type: review-response
title: ChannelTypeSchema.anchor typed as string
finding: types/channels.ts ChannelTypeSchema.anchor is `string`, though the server and OpenAPI enumerate 'family' | 'face'. cardinality and persist are already literal unions. Make anchor one too, so a switch over it is exhaustiveness-checked.
severity: nit
resolution: ChannelTypeSchema.anchor is now the literal union family or face.
status: addressed
---
