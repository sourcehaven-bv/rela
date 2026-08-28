---
id: RR-AMBIG
type: review-response
title: "inertWidgetWarnings was silent on an unvalidatable ambiguous source"
severity: minor
status: addressed
finding: "widgetSectionDef returned nil for three distinct situations collapsed into silence, and its godoc justified this with 'ValidateConfig already errors on that separately'. True for an unknown collection; NOT true for a collection whose target type cannot be determined statically (determineTargetType returns empty for a relation with several to: types). ValidateConfig treats that as legal config with no error, so a widget override there got neither validation (the call site guards on sourceType != empty) nor a warning — accepted in total silence, working or not depending on the runtime entity type. That is the one case with no other signal at all."
resolution: "Added ambiguousWidgetSource to distinguish it from the other two nil cases, and a distinct warning naming the collection and explaining the override cannot be type-checked at load. Corrected widgetSectionDef's godoc to enumerate all three cases rather than assert a blanket claim that held for only one. Pinned by TestCollectConfigWarnings_WidgetOnAmbiguousSource."
---
