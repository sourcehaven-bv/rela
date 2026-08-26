---
id: RR-CAPAUT
type: review-response
title: 'Automation capabilities: blocks were dropped in the metamodel-to-internal Action conversion'
finding: |-
    convertFromMetamodel (engine.go) builds the internal automation.Action from metamodel.AutomationAction with a hand-written field list. It copied AllowACLBypass but NOT Capabilities, so an automation's capabilities: block was discarded one hop before it could reach LuaToExecute and the runtime.

    Same observable failure as RR-CAPSCH and same class of cause (a hand-copied field list), on a different surface. Found while fixing RR-CAPSCH, which is itself the point: two of the five hand-copy sites had already lost the grant.
resolution: |
    The conversion now carries Capabilities alongside AllowACLBypass, and — more importantly — every config-to-runtime translation was consolidated behind metamodel.Capabilities.Fields(). Adding a capability changes that signature, which is a COMPILE error at each consumer rather than a silent per-surface omission.

    Pinned by TestCapabilitiesSurviveActionConversion; mutation-verified by removing the field again.
severity: critical
status: addressed
---
