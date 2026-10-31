---
id: RR-ZE1C99
type: review-response
title: onRevealed drops the URL hash
finding: 'frontend/src/components/entity/channels/ChannelsSection.vue:201: router.replace({ query }) builds a location without a hash, so any #fragment on the entity page is lost after a reveal. Pass { ...route, query } or { query, hash: route.hash }.'
severity: nit
resolution: 'ADDRESSED router.replace({ query, hash: route.hash }); test "the reveal keeps the hash"; router mock carries hash.'
status: addressed
---
