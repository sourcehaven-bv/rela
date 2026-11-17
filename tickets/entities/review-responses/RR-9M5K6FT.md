---
id: RR-9M5K6FT
type: review-response
title: db import-fs accounting missing the audit tables
finding: TestImportFS accounting did not list audit_log, audit_subjects and audit_anchor.
severity: minor
resolution: Added the three tables with the reason they are not copied.
status: addressed
---
