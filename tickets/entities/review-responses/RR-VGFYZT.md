---
id: RR-VGFYZT
type: review-response
title: ChannelTypeView godoc contradicts serving in on the descriptor
finding: internal/realtime/describe.go documents ChannelTypeView as carrying "no gate, role or condition; the affordances on each channel say what a caller may do", and SessionDescription deliberately omits transitions so "a stale descriptor cannot disagree with the channel". in is a state gate on the static descriptor. The plan does not touch either comment. Update the ChannelTypeView godoc, and say in the v1 doc, the OpenAPI description and the TS comment that in explains the rule and that _actions.post and can_post stay authoritative. Otherwise the next client computes postability from a cached descriptor.
severity: minor
resolution: "addressed\tStep 2 updates the ChannelTypeView godoc; the v1 doc, OpenAPI description, TS comment (via v1) and guide say in explains the rule and _actions.post and can_post stay authoritative."
status: addressed
---
