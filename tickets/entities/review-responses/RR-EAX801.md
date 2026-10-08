---
id: RR-EAX801
type: review-response
title: Validation scripts lose history
finding: lateGatedReader did not forward the history methods, so validation rules on a versioned backend were told the backend has no history.
severity: significant
resolution: lateGatedReader forwards EntityVersions and EntityVersion; TestLateGatedReader_ServesHistory.
status: addressed
---
