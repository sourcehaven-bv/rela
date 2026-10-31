---
id: RR-GOXRFJ
type: review-response
title: Earlier guard that is both undecidable and reads a left-out property is hidden, not refused, and that is not pinned
finding: 'buttons.go:415 runs the undecidable check first, so an earlier entry with when: session.reminders.x > 0 and message.feedback ~= '''' that admits a clicker''s role hides the button for them instead of being refused. That is safe, but it differs from the left-out rule in the docs. Add a test case that pins the chosen behaviour.'
severity: nit
resolution: "addressed\tAdded a TestButtons_ComposeGuards load case: an earlier lead-only entry with an undecidable guard that also reads a left-out property hides the button (hidden [0]) instead of refusing it. The guide now states this."
status: addressed
---
