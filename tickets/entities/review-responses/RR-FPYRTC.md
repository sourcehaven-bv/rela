---
id: RR-FPYRTC
type: review-response
title: No cap on mention targets per body
finding: No body size limit applies; a body with many distinct id-shaped spans writes one index entry per span on every save.
severity: minor
resolution: mentions.Targets caps at 1000 targets per body, sorted for determinism; mentions.Index does the same and logs once per persisted write. Forward links use the uncapped mentions.All. Conformance TargetCap covers the cap.
status: addressed
---
