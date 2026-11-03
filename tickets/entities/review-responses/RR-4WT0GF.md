---
id: RR-4WT0GF
type: review-response
title: The leftover type button ignores the caller's roles, the same flaw _actions.buttons fixes
finding: 'decisionViews groups entries without regard to roles (frontend/src/components/entity/channels/channelView.ts:390-437). With entries [0] roles [lead] without a button and [1] a button for everyone, a member sees the preset and also a type button whose toLabel is the lead-only target. The member''s post can never reach that target. The plan''s argument under ''Let the SPA decide'' applies here too. Generalize _actions.buttons to _actions.entries: the {message_type, entry} pairs the caller admits and that no unguarded earlier entry the caller admits shadows, with button entries also passing the preset shadow rule. The SPA then builds presets from the button entries in that list and the type button from the remaining ones. The cost is the same single pass. Also keep opensForm set when any earlier person entry of the type is guarded, not only when one of the remaining entries is, so that a one-click type button cannot be taken by an earlier guarded preset entry.'
severity: significant
resolution: _actions.buttons is generalized to _actions.entries, and the SPA builds presets from listed button entries and the type button from the caller's listed entries without a button. opensForm stays set when an earlier person entry of the type, a button entry included, is guarded.
status: addressed
---
