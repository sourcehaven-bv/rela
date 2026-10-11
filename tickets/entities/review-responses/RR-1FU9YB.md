---
id: RR-1FU9YB
type: review-response
title: 'Design: no DB backstop for purge'
finding: A purge path that skips the check would leave dangling tags.
severity: minor
resolution: FK version_tags.vseq to entity_versions(vseq) ON DELETE RESTRICT (R10).
status: addressed
---
