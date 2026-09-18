---
id: RR-SQ1AJR
type: review-response
title: Fifteen copy-pasted cache blocks invite consistency defects; extract a composite action
finding: The change repeats a near-identical cache block at 15 call sites with one field varied. Every defect found in this review except the Playwright comment is a consistency defect between those copies — the critical one (RR-S45X2X) is literally 'one of the fifteen is missing a line'. A local composite action (.github/actions/setup-go-cached) would collapse each site to three lines and make the module/build split and the resolved-version and ImageOS fixes structural rather than per-site discipline. Being first-party and in-repo, it raises no supply-chain question.
severity: minor
reason: Deferred to a follow-up, not declined. The four consistency defects the reviewer found are now fixed at all 15 sites, so the immediate risk is discharged; extracting the action is a refactor of working configuration that would be better validated on its own, against a CI run whose timings are not also absorbing the changes measured here. Filed as a tier-2 candidate alongside sharding the Test job.
status: deferred
---
