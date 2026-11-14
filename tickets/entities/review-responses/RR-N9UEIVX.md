---
id: RR-N9UEIVX
type: review-response
title: Unset decision recovery path and missing test items
finding: An entity with an unset machine property can never move. The predicate rule glob and the JSON kind need care.
severity: minor
resolution: 'The decision names the recovery path: set the value through a data migration, which bypasses the enforcer. The predicate.md glob targets internal/flowcfg/transitions*.go. A test pins kind flow and kind headless in JSON, and headless entries list their uses instead of a subject.'
status: addressed
---
