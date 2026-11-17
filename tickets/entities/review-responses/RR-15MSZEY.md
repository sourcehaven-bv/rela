---
id: RR-15MSZEY
type: review-response
title: 'Trigger ACL check: field grants ignore faces'
finding: 'fieldGranted counted a fields: grant under any type@face key for every face, so a field granted on draft passed a trigger that also writes published rows.'
severity: significant
resolution: 'fieldGranted takes the face and counts only type, "*" and type@<that face>; editorMay requires the field on every face it checks. Tests: "a field granted on another face only" and "a field granted on every face".'
status: addressed
---
