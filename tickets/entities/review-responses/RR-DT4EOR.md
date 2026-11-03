---
id: RR-DT4EOR
type: review-response
title: A keys.go hint for decision_button.compose
finding: 'Someone will write `compose: true` under `decision_button:`. The key checker will report an unknown key with no direction. Add a hint next to the existing `decision_button.label` and `decision_button.props` hints (internal/realtimecfg/keys.go:52-55): "the type''s decision button already opens the composer when a property is required; for a preset that opens it, add a button with compose: true to the entry".'
severity: nit
resolution: 'ADDRESSED New AC 14: a decision_button.compose hint next to the label and props hints in keys.go, tested in the buttonoptions_test.go shapes table.'
status: addressed
---
