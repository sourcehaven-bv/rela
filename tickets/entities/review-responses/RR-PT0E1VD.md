---
id: RR-PT0E1VD
type: review-response
title: Configure preview double-prefixes the migration error
finding: 'rela-server configure calls CheckDataEntry and prefixes every non-ConfigValidationError with ''data-entry.yaml: ''. migration.Error already starts with the file path, so the preview would read ''data-entry.yaml: data-entry.yaml uses deprecated syntax'' as one multi-line problem.'
severity: minor
resolution: 'Plan updated: configure.go unwraps *migration.Error and adds one ''data-entry.yaml: deprecated syntax: <description>; run rela migrate'' problem per detection.'
status: addressed
---
