---
id: RR-OX8G
type: review-response
title: Missing ResolveWidgetFromType update for rrule
finding: schema_output.go has ResolveWidgetFromType() which maps types to widgets. Without a case for rrule, it falls through to text. This file is missing from the plan's files-to-modify list.
severity: critical
resolution: Added schema_output.go to files list. Will add case for rrule in ResolveWidgetFromType.
status: addressed
---
