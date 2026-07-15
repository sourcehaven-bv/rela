---
id: RR-CT338H
type: review-response
title: 'JSON: add separate start/end RFC3339 fields, keep date/endDate date-only'
finding: 'Plan overloads the JSON `date`/`endDate` fields (documented YYYY-MM-DD) with RFC3339 when timed — breaks a consumer that parses them as date-only. json.go:25 promises date-only unconditionally. Reviewer option (a) is cleaner: keep date/endDate strictly date-only, add separate `start`/`end` RFC3339 fields populated only when timed. Backward-compatible for existing JSON consumers; AllDay flips too. The at-risk consumer (menubar/notification glue) couldn''t be found in-tree to confirm branching.'
severity: significant
resolution: Adopted option (a). Keep jsonEvent.Date/EndDate strictly date-only (all-day). Add `Start`/`End` (RFC3339) fields populated only for timed events, plus AllDay from the flag. Existing date-only consumers unaffected. Update json_test round-trip to cover the timed shape (AllDay=false, Start set, Date empty-or-date-only).
status: addressed
---
