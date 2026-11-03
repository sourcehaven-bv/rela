---
id: RR-GVZEOTE
type: review-response
title: Several blocks all headed Channels
finding: Each block renders h2 Channels (ChannelsSection.vue:250-252); unique ids still give regions the same name.
severity: minor
resolution: When a page has more than one block, a block holding one channel type is headed with that type's label; otherwise the heading stays Channels, so the default page is unchanged.
status: addressed
---
