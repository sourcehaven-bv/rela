---
id: RR-AM8NG4
type: review-response
title: datamigration rule still describes the source as opened "with the fsstore index cache off"
finding: '.claude/rules/datamigration.md:101: "The source is opened through `storage.ReadOnlyFS` with the fsstore index cache off, so the import cannot write to it." The rule file is loaded into agent context for fsimport work, so a stale mechanism here propagates. The read-only FS alone now guarantees that the import cannot write. Change it to "The source is opened through `storage.ReadOnlyFS`, so the import cannot write to it."'
severity: minor
resolution: .claude/rules/datamigration.md now says only that the source is opened through storage.ReadOnlyFS.
status: addressed
---
