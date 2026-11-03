---
id: RR-7NXI16
type: review-response
title: Chosen refs are keyed by property name, not by message type
finding: 'RlChannelComposer.vue:113, 275, 216-221. Drafts are per type (`drafts[type][prop]`) but `refs` is a flat map by property name. Two message types that share a ref property name (owner, assignee: likely) leak into each other. Pick PER-7 as owner on type A, switch to type B: B''s owner field shows the PER-7 chip while drafts[B].owner is undefined. On an optional field the post goes out without the owner the user can see on screen, which is silent data loss. On a required field the user gets "Required." next to a visibly chosen entity. reset() also clears every type''s refs but only the current type''s draft, so other drafts keep bare ids without chips. Fix: key refs as `refs[type][prop]`, or store the ChannelReadableRef next to the id in the draft. Clear them together in reset(). Add a type-switch story.'
severity: significant
resolution: Chosen refs are now kept per message type next to the drafts and reset together (story RefsPerType).
status: addressed
---
