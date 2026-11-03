---
id: RR-K7MMN6
type: review-response
title: Docs misstate who may post and when the channel opens directly
finding: 'docs/realtime.md and GUIDE-realtime.md, section ''In the web app'': ''Message markdown is limited, because any reader may post it'' is wrong, since only a user whose message-type `post` rule admits them may post (frontend/CLAUDE.md states this correctly). ''When the entity has a single one channel type and nobody can open more channels'' is also wrong: the code opens directly when there is one row and the current user cannot create (ChannelsSection.vue:59), which includes a single `many` channel. The ''**New channel** button'' is labelled by create_label or ''New <label>''. Fix the three sentences.'
severity: minor
resolution: The web app section of docs/realtime.md and GUIDE-realtime.md now says that users admitted by a message type post rule write the markdown, that the channel opens directly when there is exactly one channel and the user may not create another, and that the button shows create.label or New channel.
status: addressed
---
