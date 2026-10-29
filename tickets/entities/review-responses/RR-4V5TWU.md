---
id: RR-4V5TWU
type: review-response
title: Row ceiling uses a per-type channel cap that does not exist; make it a constant and clamp it
finding: Plan step 2 says that MaxRows = len(anchors) * the type's channel cap. There is no per-type cap. The only cap is Limits().ChannelsPerEntity (internal/realtimecfg/compiled.go:53), which an operator can lower (validate.go:102), possibly below the number of channels that already exist. That makes ErrTooManyRows reachable, and the count it compares is taken before the read-role gate, so whether _busy appears on a page would depend on channels the caller cannot read. The plan should use the hard constant realtimelimits.MaxChannelsPerAnchor (limits.go:25) for many and 1 for one, and clamp the product to channels.MaxWaitingRows. Today per_page max 100 (internal/dataentry/api_v1.go:2650) times 200 is exactly 20000 (internal/channels/waiting.go:20), so a larger page would fail CheckWaitingQuery with ErrInvalidKey and silently drop _busy.
severity: minor
resolution: The ceiling is now len(anchors) for one and len(anchors) * realtimelimits.MaxChannelsPerAnchor for many, clamped to channels.MaxWaitingRows. Security and Edge cases note that stored channels cannot reach it, so the pre-gate count never decides whether _busy appears.
status: addressed
---
