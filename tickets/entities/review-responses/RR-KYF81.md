---
id: RR-KYF81
type: review-response
title: Validation currently requires script field — needs set XOR script logic
finding: 'validateActions (validate.go:836-838) requires non-empty script field and .lua extension for ALL actions. Adding set-type actions (no script) will fail existing validation. Must rewrite to: set XOR script. Also validate key/label are present when action is referenced by a list.'
severity: significant
resolution: 'Plan updated: rewrite validateActions to require set XOR script. Script-only actions (sidebar) don''t need key/label. When action is referenced by a list''s actions field, validate key and label are present. Properties in set validated against referencing list''s entity type.'
status: addressed
---
