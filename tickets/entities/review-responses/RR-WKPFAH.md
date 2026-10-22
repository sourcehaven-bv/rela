---
id: RR-WKPFAH
type: review-response
title: Edge cases are listed but have no acceptance criterion or test
finding: 'The Edge Cases section names these, but no AC or test covers them: re-entry (to == current state: generation rises, entered_at resets, a notice is posted), a reply of a driving type, independent sessions on two channels of a cardinality-many type, a guard that reads an absent optional property (fails closed and does not error the post), and the seq 999 to 1000 segment boundary. Put the segment case in file_test.go with newStore and a small segSize, not in channelstest.RunAll, where it would make 999 appends on every backend including postgres.'
severity: minor
resolution: 'Plan updated: each edge case has its own test: re-entry, reply, two channels of a many type, absent optional property, and the segment boundary in file_test.go.'
status: addressed
---
