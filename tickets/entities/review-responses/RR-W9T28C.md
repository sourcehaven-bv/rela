---
id: RR-W9T28C
type: review-response
title: PropError need not be exported, and its text repeats the property name already in the key
finding: 'Only realtimecfg reads the per-property refusal; realtime.validateProps uses Error() text alone. An exported PropError with an unexported err field adds API surface that no consumer needs. Make it unexported (propError), or document why it is exported. The load reason also repeats the name: key `...post.props.short` with reason `property "short": must be at most 3 characters`. If the error carries the bare reason separately (Prop, Reason), literalPost can print `must be at most 3 characters` at the per-property key, and Error() keeps the fire-time text byte-identical. Decide this in the plan; AC 3 currently pins the redundant form.'
severity: minor
resolution: "addressed\tpropError is unexported and carries a bare reason for the per-property key; Error() keeps the fire-time text byte-identical; AC 3 pins the bare reasons."
status: addressed
---
