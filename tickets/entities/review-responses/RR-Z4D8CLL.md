---
id: RR-Z4D8CLL
type: review-response
title: Mention and reply of one message notify twice
finding: A message listed as both mention and reply produced two candidates with the same id.
severity: minor
resolution: 'diffInbox drops duplicate ids before slicing. Test: counts a message listed as both mention and reply once.'
status: addressed
---
