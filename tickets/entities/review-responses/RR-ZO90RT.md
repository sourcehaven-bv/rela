---
id: RR-ZO90RT
type: review-response
title: 'channels: presentation does not reach an open SPA on refresh'
finding: 'Plan D2 says changes to either file reach the SPA through `refresh`. channelSchema.ts reloads the descriptor on refresh, but the presentation lives in stores/schema.ts (channelPresentations, loaded from /_config), and that store does not listen to refresh (only App.vue boot and Retry call load/reload). A data-entry.yaml edit to channels: therefore needs a page reload. forms: already behaves this way, but this PR''s plan claims otherwise, and a realtime rename plus a presentation fix leaves the SPA mixing a new descriptor with an old presentation. Either reload the config part of the schema store on refresh, or correct the plan and docs so they do not promise it.'
severity: minor
resolution: useChannelSchema reloads the channels presentation from /_config together with the descriptor on refresh, keeping the held presentation on failure, and the docs and plan say so.
status: addressed
---
