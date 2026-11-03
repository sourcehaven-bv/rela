---
id: RR-3MGH51
type: review-response
title: today() refusal now hidden behind literal problems
finding: internal/realtimecfg/validate.go:1013 - the today() check is about the message type's when:, not about the literal, yet it now runs only after the literal policy and NormalizeProps pass. Before, literalMessage reported it even when a prop was bad. An operator with a bad literal on a today()-using type fixes the literal, reloads, and only then learns the type can never be a trigger's or reminder's post. Move the today() check before the policy loop (it needs no props), so the unfixable problem shows first.
severity: minor
resolution: "addressed\tliteralPost now refuses a message type whose when: calls today() before the literal policy and NormalizeProps, and TestParse_TriggerLiteralOrder pins that a bad ref literal on a today() type reports only the today() problem."
status: addressed
---
