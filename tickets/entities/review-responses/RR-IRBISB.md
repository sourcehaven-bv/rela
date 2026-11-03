---
id: RR-IRBISB
type: review-response
title: RlRefChip binds app-supplied href without a scheme guard
finding: 'frontend/packages/rela-components/src/components/channel/RlRefChip.vue:461 binds :href="href" and RlChannelMessage.vue:128-130 passes whatever entityHref returns. Vue 3 does not neutralize javascript: or data: URLs in :href, so the library relies entirely on the app building the href from a server-resolved id (plan R3/R14). That holds for the planned ChannelsPanel, but RlRefChip is an exported generic component. Defense in depth: in RlRefChip (or hrefFor) render the chip as the button variant when href does not start with "/", "#" or "?" (or does not parse as same-origin), and state the contract on the entityHref prop doc ("must return an in-app path built from ref.id; never author data"). Not exploitable in this diff.'
severity: nit
resolution: RlRefChip now ignores an href that is not a same-page relative path and renders the button instead, and the entityHref docs state the contract (story UnsafeHref).
status: addressed
---
