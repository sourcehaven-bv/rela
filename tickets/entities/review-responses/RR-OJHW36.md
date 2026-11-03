---
id: RR-OJHW36
type: review-response
title: Wire golden samples cover only the happy path
finding: channels_wire_golden_test.go:117-140 samples one plain text message. The 'real answers use only mirrored keys' half of wireFields.test.ts never sees a removed message, a pending or done action, a promoted ref, a restricted ref, an ephemeral signal or a message with op affordances. Those are the shapes most likely to carry a field the mirror lacks. Add a removed message, an action run, a restricted ref and an ephemeral post to the env before sampling. jsonFields also does not flatten embedded structs (it would emit the type name), which is a trap for the next wire type that embeds.
severity: minor
resolution: The wire golden now includes an own message with deadlines, a restricted ref, done and pending actions, a removed message and an ephemeral signal, and the SPA test checks the keys of every nested wire value.
status: addressed
---
