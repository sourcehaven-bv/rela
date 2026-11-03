---
id: RR-1G2SCG
type: review-response
title: SetSchemas ignores soft-delete marks and id collisions
finding: 'internal/store/fsstore/schemas.go:72. Found entries are added without checking s.marked, so a file whose id belongs to a soft-deleted family is resurrected into the live index. createEntity refuses that id through markedTaken. The ''already in index, continue'' branch also silently hides a real cross-directory id collision (same stem in two type dirs) behind the re-declared-type explanation. Fix: skip with slog.Warn when markedTaken(s, meta.ID, ""), and warn when the existing entry''s Type differs from meta.Type.'
severity: minor
resolution: SetSchemas skips ids of soft-deleted families and warns on id collisions between type directories.
status: addressed
---
