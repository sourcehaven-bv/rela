---
id: RR-VF5NYV
type: review-response
title: The base model was exported with the base's exporter pin
finding: The base worktree used develop's justfile and only the head's config.json, so an exporter bump would make the two models differ for reasons other than code.
severity: significant
resolution: The base step copies the head's .archplan settings (minus local files) and the head's justfile into the base worktree, as archplan's own --base does.
status: addressed
---
