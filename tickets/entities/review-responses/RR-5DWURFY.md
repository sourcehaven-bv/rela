---
id: RR-5DWURFY
type: review-response
title: AC7 passes without the announce change
finding: A single notice append is a full row on arrival, so the current announce already reads it; the new code path (notice folded on arrival) is untested and a double announcement is not checked.
severity: significant
resolution: 'Plan: test liveAdds([notice, progress]) announces only the notice, and a notice then a progress append announces once.'
status: addressed
---
