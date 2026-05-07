---
id: RR-E8VBI
type: review-response
title: properties_unset doesn't validate against closed schema (mirror inconsistency)
finding: |-
    Per-edge meta_unset validates that keys exist in RelationDef.Properties (api_v1.go:651-657). Entity-level PropertiesUnset (line 567-569) does `delete(staged.Properties, k)` for any key the user names — no validation. Typo 'titel' instead of 'title' silently succeeds; user thinks they cleared title but didn't.

    Fix: same closed-schema gate at the entity level. Reject unknown PropertiesUnset keys with a 422 just like meta_unset does.
severity: minor
resolution: Added closed-schema check on req.PropertiesUnset — unknown property keys return 422 just like meta_unset does. Test TestV1Patch_PropertiesUnsetUnknownKey confirms a typo'd property name is rejected with a clear error.
status: addressed
---
