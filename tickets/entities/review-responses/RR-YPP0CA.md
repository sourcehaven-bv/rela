---
id: RR-YPP0CA
type: review-response
title: dataentry may not import realtimecfg, and the descriptor duplicates realtime.messageTypes
finding: '§1 and §2 build the handler in `internal/dataentry/channels_schema.go` "from the compiled config snapshot". `.go-arch-lint.yml` lets dataentry depend on realtime, channels and realtimelimits, not on realtimecfg, so `just arch-lint` fails. The service also already describes message types: `call.messageTypes` in internal/realtime/read.go:207 (Label, Persist, the `Reply` flag, `TTLSeconds` via `signalTTL`, which caps at MaxEphemeralTTL, and PropsSchema). A second builder would drift on the TTL cap and on reply rules. Fix: add `realtime.Service.Describe(ctx)` (or a pure function in realtime) that returns view types built from one config snapshot and reuses `signalTTL`. dataentry maps it to apiwire the way channels_wire.go does. Carry `reply` (ephemeral types cannot be replied to) and the real labels: Gate.Label for actions and `create.label`.'
severity: significant
resolution: The descriptor comes from a new realtime.Service.Describe that reuses messageTypes, signalTTL and the reply flag; dataentry only maps it to apiwire and never imports realtimecfg (R4).
status: addressed
---
