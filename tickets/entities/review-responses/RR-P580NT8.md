---
id: RR-P580NT8
type: review-response
title: Late $user fill does not rerun the dry-run
finding: When /_me answers after mount, the create dry-run had run without the value, so policy verdicts that depend on it stayed stale.
severity: significant
resolution: 'fillLateUserDefaults calls scheduleStagedAffordances after a fill. Test: runs the dry-run again with the late value.'
status: addressed
---
