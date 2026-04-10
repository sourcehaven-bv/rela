---
id: RR-U607K
type: review-response
title: Script field JSON tag lacks omitempty
finding: Action.Script has json:"script" without omitempty (config.go:88). For set-type actions Script is empty. Safe to add omitempty since actions aren't in V1Config yet and frontend sidebar only uses action IDs, never reads the script field from config.
severity: minor
resolution: Will add omitempty to Script json tag. Safe since actions aren't in V1Config yet and no frontend consumer reads the field.
status: addressed
---
