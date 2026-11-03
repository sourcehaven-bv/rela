---
id: RR-IQDHGX
type: review-response
title: Delete guard runs before in-Tx cascade authorization
finding: Cascade relation and owned-delete authorization run only in the Tx, so an unauthorized caller causes a tombstone.
severity: significant
resolution: 'Plan updated: .ignored/t3-plan.md section 6 (Design review folded in) redesigns this point; implementation follows that section.'
status: addressed
---
