---
id: RR-5C3EMJ
type: review-response
title: Face and world are absent from the SPA design
finding: '§3 `ChannelTarget` and `api/channels.ts` never mention face or world. The channel routes and live paths key on the anchor''s face (`channels.Anchor{ID, Face}` in livepath.go:199; TypeFaceRequired is a 422) and accept `world` (liveChannelKeys). A bare id for a faced type names the zero-face row (DEC-NPZICR). Entity links in the renderer and the ref chip need the page world (`renderMarkdown`''s `world` option). Fix: build every channel URL and the live path from the entity''s own ref (id plus face) and the page `?world=`, the same as EntityDetail''s entity subscription. Add a faced-type case to useChannel.test.ts and to the e2e spec (faced fixtures exist: e2e/tests/faced-project.ts).'
severity: significant
resolution: Every channel URL and live path is built from the entity's own id and face plus the page world, with faced-type cases in vitest and e2e (R14).
status: addressed
---
