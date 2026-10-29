---
id: RR-ANFACM
type: review-response
title: Inbox mention or reply rows lead to an invisible message
finding: The plan documents the mention risk but understates it. An inbox mention or reply row opens the channel only, with no seq deep link (ChannelsSection.vue, deeplink test), so the reader lands in a channel where the message that mentioned them is hidden and has no way to find it except the toggle. Change the docs rule to cover both mentions and replies to people, and record the gap under Q5 so the follow-up that teaches the server about quiet types handles it.
severity: minor
resolution: The docs rule now says display is only for types that people cannot post and that do not mention or reply to people, and the inbox gap is recorded under Q5 and the follow-up "Server-known quiet message types (unread, inbox)".
status: addressed
---
