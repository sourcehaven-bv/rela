---
id: RR-I5S4NK
type: review-response
title: 403 body reflects the attacker-supplied principal name unnecessarily
finding: router.go builds the 403 detail as "'"+p.User+"' is an internal identity and cannot be asserted by a request.", echoing up to 256 attacker-chosen runes back to the caller. It is JSON-encoded via writeV1Error's json.Encoder so there is no injection, and control chars are already stripped by sanitizeUser, and per CLAUDE.md naming the rule is correct (the config is not a secret). But the VALUE is not needed to convey the rule -- a fixed message naming the 'system:' prefix is equally actionable for the operator without making the endpoint a reflection gadget for whatever consumes that problem+json downstream. The log line already carries the value, which is the right place for it.
severity: minor
resolution: 'The 403 body now names the RULE without echoing the VALUE: "A ''system:'' prefixed identity is internal to rela and cannot be asserted by a request." The prefix is interpolated from principal.ReservedPrefix so the message cannot drift from the constant. Equally actionable for an operator debugging a proxy, without reflecting up to 256 attacker-chosen runes into the response. The attempted name remains in the WARN log line, which is where it belongs. Verified live: the endpoint returns the fixed message.'
status: addressed
---
