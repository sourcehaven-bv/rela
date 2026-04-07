---
id: RR-2BAD
type: review-response
title: 'S5: containedProjectPath conflated ''file does not exist'' with ''outside project'''
finding: filepath.EvalSymlinks fails on ENOENT, so any non-existent path returned errPathOutsideProject and the handler returned 403. Operators debugging a broken open-file UX would chase ghost traversal warnings when the real issue was 'file is gone'.
severity: minor
resolution: Split into errPathOutsideProject (real traversal) and errPathNotFound (structurally inside project but missing on disk). handleOpenFile now returns 404 for the not-found case and 403 for traversal. Containment check happens against the unresolved abs path before EvalSymlinks so we can distinguish the two.
status: addressed
---
