---
id: RR-9RESTU
type: review-response
title: Quiet rows can hide a message action a person must use
finding: 'A message type can declare actions (the PoC''s answer has record). ChannelMessageView.actions (types.ts:230-236) mixes the operations edit, delete, remove and release with those type actions. The kept-row rule (AC 4) covers decisions and choices but not type actions, so display: hidden on a type with an action hides its button. Change the plan: keep a row whose actions include a message-type action (not one of the four operations), or document that display must not be set on a type with actions.'
severity: minor
resolution: A row whose actions include a name other than edit, delete, remove and release is now a kept row (AC 4, step 4). Added a kept case and a negative case for operation-only actions.
status: addressed
---
