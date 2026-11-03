---
id: RR-JHJWZC
type: review-response
title: The 'docs example' test is a hand-made copy of the docs YAML
finding: 'channels_test.go:70-71 copies the YAML into a constant with the comment ''Keep the two equal''. Nothing fails when docs/data-entry.md or GUIDE-data-entry.md changes, so AC 10 (''the documented YAML example is a case'') holds only on the day it was written. Fix: in the test, read docs/data-entry.md (relative to the package, as the frozen-typography test reads both files), take the first ```yaml block after ''### Quiet message types'', and validate that block. The test then fails when the heading or the block goes missing.'
severity: minor
resolution: Fixed. The docs example case reads docs/data-entry.md, takes the first yaml block after "### Quiet message types" and validates it. The test fails when the heading or the block is missing or unclosed. The hand-made constant is gone.
status: addressed
---
