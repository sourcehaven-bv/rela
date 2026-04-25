---
id: RR-2OGRV
type: review-response
title: 'Missing test: script: + edit: combination'
finding: All four new edit-block cases in validate_test.go use Command. The validator doesn't care which renderer, but a future refactor coupling them would slip past. Add a row with Script + Edit.
severity: minor
resolution: Added 'edit block with script renderer' row to TestValidateConfig_Documents, exercising the Script + Edit path.
status: addressed
---
