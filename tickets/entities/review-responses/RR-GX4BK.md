---
id: RR-GX4BK
type: review-response
title: Inline metamodel has no automations — limits future tests
finding: feature/bug/task schema lacks automations/validations. Cannot port a test that exercises workflow-checklist auto-creation or validation rules. Add a comment flagging this.
severity: minor
resolution: Added comment block in fixtures.ts above METAMODEL_YAML explicitly warning not to point at the tickets/ dogfood project and flagging the no-automations limitation.
status: addressed
---
