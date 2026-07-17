---
id: RR-GBL60P
type: review-response
title: Empty/required datetime handling + relation-property render path unspecified
finding: 'Two gaps: (1) Empty datetime: <input datetime-local> yields '''' when cleared; the validation arm must early-return OK on ''''/absent unless required, and the widget should omit the key rather than emit '''' (mirror how the date arm handles empties - verify it does). (2) PropertyDef is shared by entity and relation schemas; datetime must render in the data-entry relation-property modal (a separate render path from the entity form), and filter/sort/affordances run on relations too. Add explicit verification that DatetimeWidget is reachable from the relation modal.'
severity: minor
resolution: 'Accepted into plan. (1) Empty datetime: validation arm early-returns OK on ''''/absent unless required (mirror date arm - verify it does); widget omits the key rather than emitting '''' when cleared. (2) Add explicit test/verification that DatetimeWidget renders in the data-entry relation-property modal (separate render path from entity form) since PropertyDef is shared by entity+relation schemas.'
status: addressed
---
