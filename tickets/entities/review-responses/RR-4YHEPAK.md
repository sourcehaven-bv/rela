---
id: RR-4YHEPAK
type: review-response
title: Stale proposal overwrites concurrent edits
finding: Between propose and accept another user may change a proposed property. Accept PATCH would overwrite it. Dialog should re-read and skip or warn on properties whose current value changed.
severity: minor
resolution: The dialog re-reads the entity before applying and skips any property whose value changed since the proposal (planApply stale list), naming it to the user.
status: addressed
---
