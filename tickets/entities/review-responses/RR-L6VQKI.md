---
id: RR-L6VQKI
type: review-response
title: Fail-closed on a partial List union was undocumented
finding: layered.List fails the whole call when either layer errors, so a transient database fault takes down a listing that disk could have served completely. That is defensible for Load, where the layers compete, but weaker for List, where they are additive. The doc explained the union and never said what a partial union means, inviting the next reader to 'fix' it into a partial-success path.
severity: minor
resolution: 'Documented the choice and its reasoning on layered.List: there is no partial union because a caller cannot distinguish a half-listed directory from a complete one, so an automation missing because the database hiccuped would look exactly like an automation that was never configured. Behaviour unchanged; the decision is now written down.'
status: addressed
---
