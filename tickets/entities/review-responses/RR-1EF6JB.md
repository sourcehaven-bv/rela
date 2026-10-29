---
id: RR-1EF6JB
type: review-response
title: Composer Send shows 'Sending...' while a decision is in flight
finding: Approach 9 passes `:submitting="submitting || !!deciding"`. In RlChannelComposer.vue the Send button binds `:loading="submitting"` with pending label 'sending' (and disables edit Cancel). So while the user clicked Approve, the composer's Send spins and announces 'Sending...' as if their draft were being sent. Add a separate `disabled`/`busy` prop to the composer that blocks submit without the pending treatment, or accept it and say so in AC 5.
severity: minor
resolution: The composer gets a new blocked prop that stops submit without the Sending treatment, used instead of submitting while a decision is in flight.
status: addressed
---
