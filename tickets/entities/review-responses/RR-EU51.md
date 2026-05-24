---
id: RR-EU51
type: review-response
title: ctxRecorder.called field is theater — not independent of marker check
finding: 'runtime_test.go:2255-2261, 2379: the `called` flag is set-but-effectively-unused as a regression signal. If called is false, marker is also nil, so the second assertion would trip too. The two checks aren''t independent. Drop `called` or restructure.'
severity: minor
resolution: Removed the `called` field entirely. Replaced with `len(rec.calls) == 0` check for the 'binding did not invoke any spied collaborator' case, which is now the only assertion left. The per-call marker check (with hasMarker bool) carries the regression signal.
status: addressed
---
