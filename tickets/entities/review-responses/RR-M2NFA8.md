---
id: RR-M2NFA8
type: review-response
title: Step 9 condition is already resolved
finding: 'Step 9 says to add a default_in type to the wire golden fixture "if no schema sample has a type with in". The session_schema sample in TestChannelWireGolden uses breachRealtime, which has a written in: [detected, contained, assessed], so in already appears and channel_wire.json changes. in_derived never appears in a sample. That is harmless, because wireFields.test.ts only flags unknown keys and takes the field set from the Go struct. State this, and drop channels_wire_golden_test.go from the files to modify.'
severity: nit
resolution: "addressed\tStep 9 resolved: fixture unchanged and channels_wire_golden_test.go dropped. Correction: channel_wire.json does not gain in, because the only in: in breachRealtime is on a reminder; only channel_wire_fields.json changes, which is enough for wireFields.test.ts."
status: addressed
---
