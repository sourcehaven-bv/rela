---
id: RR-QP73I4
type: review-response
title: stagedUnmounted must be re-checked after every await in the reset, not once
finding: stagedUnmounted is a plain let (:707). The plan cites the single check at :1207, but the reset adds two more awaits after it (loadTemplates, refreshStagedAffordances). Guard after each or the RR-2PZB write-to-dead-refs hazard reopens on the new path — the failure the existing checks at :735, :738 and :799 exist to prevent.
severity: minor
status: addressed
resolution: >-
  Plan step 3 now requires a stagedUnmounted re-check after each await in the reset, not just the existing one at :1207.
---
