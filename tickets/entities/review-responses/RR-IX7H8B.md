---
id: RR-IX7H8B
type: review-response
title: Fit with TKT-53FN0L assumes a new key that the ticket does not describe
finding: 'Approach 8 says TKT-53FN0L "will add a key to button:" and will refuse confirm together with it. The ticket describes a preset that leaves a required prop out and opens the composer prefilled (tickets/entities/tickets/TKT-53FN0L.md), which may be inferred from the preset with no new key; today such a preset is refused by NormalizeProps (buttons.go:158-168). State the rule generically instead: confirm has no effect on any button that opens the composer, whatever makes it open, and the load warns when that is known at load. Then TKT-53FN0L inherits the rule in either shape and AC 3''s required-property warning is the same rule applied to the type button.'
severity: minor
resolution: 'Approach 10 now states the rule generally: confirm has no effect on any decision button that opens the composer, whatever makes it open. TKT-53FN0L is described as a new key or inference, and the AC 4 warning is that rule applied to what the load can know.'
status: addressed
---
