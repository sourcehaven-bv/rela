---
id: RR-GAMU2LP
type: review-response
title: Empty column or lane value sent as an empty string
finding: A column or lane with value '' collects unset cards but quick-add sent the property as an empty string.
severity: minor
resolution: quickAddBody sets the column and lane property only for a non-empty value. Tested.
status: addressed
---
