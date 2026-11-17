---
id: RR-0LU1DFJ
type: review-response
title: Script pin order bypassed by remove and re-add of an output
finding: originsInOrder skipped new items, so removing an output and adding it back after the script, or inserting a new output before it, passed.
severity: significant
resolution: 'A scripted trigger''s outputs must now be exactly the items read, in order (originsIntact); only in-place edits pass. Tests: remove the output before the script, re-add the output after the script, insert an output before the script.'
status: addressed
---
