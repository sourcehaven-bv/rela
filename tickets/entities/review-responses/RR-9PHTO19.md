---
id: RR-9PHTO19
type: review-response
title: 'Test gaps: timer re-entry, scalar session value, folded aria-current count'
finding: 'No conformance case for a timer re-entry; no test for session: compact as a scalar; no test counting aria-current while folded.'
severity: minor
resolution: testSessionPrevious adds a timer re-entry that keeps Previous; TestChannelsPresentation_SessionScalar pins a readable load error; the Compact story counts aria-current in the markup.
status: addressed
---
