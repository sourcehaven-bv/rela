---
id: RR-QLQW
type: review-response
title: stripInstance hand-parses JSON; fragile against any encoder change
finding: 'stripInstance greps for `,"instance":` and chops to the next `}`. Depends on encoder ordering (instance last, no trailing comma), no nested `}` between instance and closing brace, no escaped quotes earlier matching the delimiter. None pinned. When (e.g.) an errors[] field with a `}` in a message gets added to V1Error, this silently miscompares. Fix: parse both bodies into V1Error, clear Instance, re-encode canonically. One json.Unmarshal per body, not a test bottleneck.'
severity: minor
resolution: stripInstance now parses into V1Error, clears Instance, re-encodes via json.Marshal. Robust against encoder changes and future V1Error field additions.
status: addressed
---
