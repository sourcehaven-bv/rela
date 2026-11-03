---
id: RR-WM338D
type: review-response
title: AC 6's face subtest needs a way to move PAGE-1@draft
finding: busyPolicy gives every reader the published world only (busy_rows_test.go:124-131), so neither alice nor bob can post on PAGE-1@draft through the service. State in the test plan that the move is made by a direct store write followed by Local.Mark (what a timer does), or by a principal with draft-world access. Also add a positive control for the same round, such as a draft-world board that does wake.
severity: nit
resolution: 'Adopted: the face subtest makes the draft move by a direct store write plus Local.Mark, and uses a run move on TKT-1 that replays bob''s second busy=run board in the same tick as the positive control.'
status: addressed
---
