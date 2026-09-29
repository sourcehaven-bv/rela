---
id: RR-BC5F0R
type: review-response
title: Migration 0018 lock comment wrong and drops precede the create
finding: The DROP INDEX statements take ACCESS EXCLUSIVE on entities until commit and ran before CREATE INDEX. Readers were blocked for the whole build. The comment said only writers block.
severity: minor
resolution: CREATE INDEX now runs first and the three drops last. The comment states SHARE for the build (writers block) and ACCESS EXCLUSIVE for the drops until commit.
status: addressed
---
