---
id: RR-35Q8CW
type: review-response
title: Design doc puts the five switches on Writer
finding: The design places the switches on access.Writer while the plan uses Mode; PR 6 must obtain Mode from the access service rather than ModeOf on acl.ACL.
severity: nit
resolution: 'Recorded in the TKT-0NODIZ notes: Mode replaces Writer for these decisions and PR 6 obtains it from the access service so the arch-lint deny can hold.'
status: addressed
---
