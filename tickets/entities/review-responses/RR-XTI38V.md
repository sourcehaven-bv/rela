---
id: RR-XTI38V
type: review-response
title: First-page test does not pin that the reveal wins over the mount scroll
finding: 'frontend/src/components/entity/channels/ChannelsSection.deeplink.test.ts:312 says "The reveal ran after the mount''s scroll to the bottom and won" and checks that scrollIntoView was last called with block center. RlChannel''s mount scroll is scrollToBottom, which sets log.scrollTop (RlChannel.vue:290-291) and never calls scrollIntoView, so the assertion passes in either order. The only other scrollIntoView call is ChannelsSection''s section scroll. Either assert on log.scrollTop or a spy on scrollToBottom, or reword the comment so it states what is checked: the section scroll does not run after the reveal.'
severity: minor
resolution: 'ADDRESSED reworded the comment to state what is checked: the section scroll does not run after the reveal, and the order against scrollToBottom (which sets scrollTop) is not shown.'
status: addressed
---
