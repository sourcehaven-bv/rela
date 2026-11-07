---
id: RR-NPAFEHU
type: review-response
title: 'Code: migration test does not rerun 0042'
finding: TestMigration0042_Elements called Migrate twice at version 42, so the second call ran nothing.
severity: minor
resolution: The test resets schema_version to 41 between the calls, so 0042 really runs twice.
status: addressed
---
