---
id: RR-7ZWWK3C
type: review-response
title: The property cannot see a stranded hold
finding: The loss check excused a stranded hold because the later value landed.
severity: minor
resolution: 'Added an end-of-run check: every field unsavedFields() names shows an error. Removing the release rules fails it (''A holds the body without an error''); removing the holds fails the loss check.'
status: addressed
---
