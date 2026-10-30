---
id: RR-RNYB5T
type: review-response
title: MessageType.In doc comment says file order
finding: 'internal/realtimecfg/compiled.go:463-465 documents In as "in file order". A derived In follows the order of session.states. Update the comment: "the states as listed in in:, or, when derived from session.offer, in the order of session.states".'
severity: nit
resolution: The plan now changes the In doc comment to "the states as listed in in:, or, when derived from session.default_in, in the order of session.states".
status: addressed
---
