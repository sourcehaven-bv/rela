---
id: RR-50DDZE
type: review-response
title: 'The refusal for an authored in: under offer: transitions gives advice that is now incomplete'
finding: 'AC 3 keeps the TKT-QYFLRH refusal of an authored in: that misses a keyed state. Its text says "add <state> to message_types.<mt>.in" (internal/realtimecfg/message_in.go:83-85). Under offer: transitions, dropping the in: is the simpler fix, because the derived in: is exactly the keyed states. Change: when offer is set, append "or drop in to take the keyed states from session.offer". Pin the text in the TestOffer_Derive case "authored in missing a keyed state is still refused".'
severity: minor
resolution: 'With default_in set, the refusal of an authored in: that misses a keyed state now ends with "or drop in to take the keyed states from session.default_in". The TestDefaultIn_Derive case pins the full text.'
status: addressed
---
