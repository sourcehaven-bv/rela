---
id: RR-8P38WS
type: review-response
title: Session notices can be removed by moderators, and rendering trusts a TriggeredBy prefix
finding: 'messageActions offers remove for any message to a moderator, so the visible record of a transition can be erased (the audit keeps it). Decide whether that is intended and test it. Also, the renderer picks out notices by a TriggeredBy prefix of "session:". Prefer the reserved type plus a system author, and have PrepareMessage refuse a TriggeredBy starting with session: unless it comes with a session step, so the store is the only writer of that prefix by construction rather than by convention.'
severity: minor
resolution: 'Plan updated: notices have no _actions, edit or remove answers 409 session_notice, and PrepareMessage refuses a session: TriggeredBy so only the store writes the marker.'
status: addressed
---
