---
id: RR-6AUIKF
type: review-response
title: Width check wording and placement differ from the display-mode check
finding: The width check used a different message form and ran from a separate loop beside the list-source checks.
severity: minor
resolution: 'validateDashboardCards calls it per card; the message reads ''has invalid width %q (valid: normal, wide, full)'', plus the column-count hint.'
status: addressed
---
