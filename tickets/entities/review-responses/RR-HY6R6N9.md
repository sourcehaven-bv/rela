---
id: RR-HY6R6N9
type: review-response
title: Definition keeps reminder and timer order from the caller
finding: NewDefinition and NewState kept caller order while realtimecfg sorts by name; a second loader could break NextTimer tie order.
severity: significant
resolution: NewDefinition sorts reminders and NewState sorts timers by name; TestNewDefinition_SortsByName pins it.
status: addressed
---
