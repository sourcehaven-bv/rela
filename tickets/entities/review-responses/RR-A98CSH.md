---
id: RR-A98CSH
type: review-response
title: 'Comments overclaimed: method count, byRun''s role, sweep aliasing, and log levels'
finding: Several comments I wrote were inaccurate. (13) 'the table is only ever touched by these three methods' — there are four, and lookup also mutates, which its own comment admits, so the file contradicted itself. (14) byRun 'so a finished run can drop its own entries without scanning the whole table' — release drops nothing and sweepLocked does scan everything. (4) sweepLocked's `live := tokens[:0]` in-place filter is correct but was the only non-obvious line in a file where every obvious line has a five-line comment. (5) mint's crypto/rand failure was logged at Warn, the same level as an operator misconfiguration, though it means the process is doomed. (15) the warning logged cmd.Label, which is optional and empty in most fixtures, rather than the command id.
severity: nit
resolution: 'Rewrote each comment to say what is true: corrected the method count and noted lookup''s single-entry delete; described byRun as the release index rather than a sweep shortcut; documented why the slice aliasing is safe and what would break it (do not hand a byRun slice out without copying), plus the O(tokens) cost the sweep pays on mint; raised the rand failure to slog.Error with its own rationale; threaded commandID through mintFileToken so the warning names the command an operator can find.'
status: addressed
---
