---
id: RR-4VCONG4
type: review-response
title: "No test pins the hamburger headroom on a page without a header slot"
finding: "The old e2e test was rewritten for lists, so the 56px/60px reservation is no longer pinned."
severity: minor
reason: "The reservation rules are unchanged by this bug; the remaining e2e tests (hamburger clears the notch) still open such pages. Not worth a new spec here."
status: deferred
---
