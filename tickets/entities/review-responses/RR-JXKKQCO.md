---
id: RR-JXKKQCO
type: review-response
title: YAML anchors and aliases fail with a confusing error
finding: Each flow is decoded on its own, so an alias across flows fails as unknown anchor.
severity: minor
resolution: 'Parse refuses any anchor or alias up front with a clear message. Test: TestParse_NullAndAlias.'
status: addressed
---
