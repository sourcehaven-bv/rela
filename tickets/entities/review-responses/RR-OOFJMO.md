---
id: RR-OOFJMO
type: review-response
title: fmt.Sprint on numeric kinds called String methods
finding: Contradicted the no-String rule; a locking String could deadlock the traced binary.
severity: minor
resolution: 'Numbers and bools are formatted from reflect values via strconv. Test: TestSummarize/int with String.'
status: addressed
---
