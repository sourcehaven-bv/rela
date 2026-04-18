---
id: RR-V7SIR
type: review-response
title: generateDataEntryConfig hand-concatenates YAML; use yaml.v3 marshaling instead
finding: 'In cmd/rela-desktop/main.go:765-841 generateDataEntryConfig builds YAML by fmt.Fprintf and %q escaping. Project already depends on gopkg.in/yaml.v3; this should marshal a typed struct. Current form has at least one latent bug: titleCase(propName) isn''t escaped if the property name contains a quote. Out of scope for TKT-AWX7V (PR only touched 20 lines for QF1012); worth a follow-up ticket.'
severity: minor
reason: Real technical debt in cmd/rela-desktop/main.go:generateDataEntryConfig, but explicitly out of scope for TKT-AWX7V (a tooling/version chore). The latent escape bug is real; filing as a separate ticket is the right call. This ticket only touched 20 lines of that function mechanically for QF1012.
status: deferred
---
