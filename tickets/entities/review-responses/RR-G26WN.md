---
id: RR-G26WN
type: review-response
title: Boundary test passed against the unmodified code — a false coverage claim
finding: 'The ''a value at exactly the cap still evaluates'' test asserted only that a 10k value matches `^a+$`. That passes on the OLD code too (which had no value cap at all), so it pinned nothing: lowering the cap to 5_000 would leave it green by accident of direction. A boundary test must fail when the boundary moves; one that passes on the broken code is worse than no test because it claims coverage that does not exist.'
severity: minor
resolution: 'Rewrote as ''the cap is exact: at-cap evaluates and does not warn, one-over is rejected'', adding `expect(warn).not.toHaveBeenCalled()` at the cap as the load-bearing assertion plus the one-over rejection. Verified by mutation testing: moving MAX_MATCH_VALUE_LENGTH from 10_000 to 5_000 now fails this test (1 failed / 40 passed); restoring it returns all 41 to green. Also removed a redundant double `prog.eval()` call flagged in the same test.'
status: addressed
---
