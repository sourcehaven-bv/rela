---
id: RR-PA3KBWR
type: review-response
title: Row cap and grouped tables underspecified
finding: Cap counts only entities; grouped rows live in groups[].rows; group_by edits move only after reload.
severity: minor
resolution: The cap counts flat rows plus every group row; grouped and columns_by_type rows are indexed for the apply path. A group_by row moves groups only after a reload.
status: addressed
---
