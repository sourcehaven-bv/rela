---
id: RR-COJ16V
type: review-response
title: A deep link to a closed channel is silently ignored
finding: 'ChannelsSection lists only open channels by default (ChannelsSection.vue:46,52-55, state ''open''). Unread rows come from the read window of live channels, and mention rows from addressed rows; InboxChannel.state can be closed or archived in both. Step 10 sets selected only "when a row with that key exists", so clicking such a row lands on the entity page with nothing open, and AC 19''s promise that rows open the channel fails without any feedback. Fix: when the deep-linked key is not in the open rows, switch the filter to ''all'' once and try again (or pass the state in the link), and add AC 19 cases for a closed channel and an archived one. The panel is also mounted below the comments (EntityDetail.vue:3157), so on a long page the opened channel is off screen: scroll it into view on a deep link.'
severity: significant
resolution: The deep link switches the filter to all once when the key is not in the open rows, scrolls the channel panel into view, and AC 23 covers closed and archived channels.
status: addressed
---
