---
id: RR-9NNLDO
type: review-response
title: Block-scalar error in dataentryconfig quotes the value with its newline
finding: 'internal/dataentryconfig/config.go:356 and dashboard.go:76 format value.Value with %q. A | block value now reaches this branch (the deliberate yamlscalar change), so the operator sees confirm: "yes\n" is not a boolean. realtimecfg trims the value for the same message (buttonoptions.go:120-121). Fix: use strings.TrimSpace(value.Value) in both messages so all three loaders print the same text.'
severity: minor
resolution: 'dataentryconfig config.go and dashboard.go print strings.TrimSpace(value.Value); the block scalar cases now assert the trimmed message (confirm: "yes" / dashboard: "yes" is not a boolean).'
status: addressed
---
