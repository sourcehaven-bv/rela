---
id: RR-29D5B1O
type: review-response
title: Task-item conversion preview refused under strict
finding: requiredFields built its candidate with the deployment mode, so under strict the missing fields it lists refused the preview.
severity: significant
resolution: Deps.advisory() is used by ValidateCreate and requiredFields; TestTaskItemConvert_RequiredFieldsComeFromTheCaller runs in both modes.
status: addressed
---
