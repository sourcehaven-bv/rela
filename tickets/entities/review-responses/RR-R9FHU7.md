---
id: RR-R9FHU7
type: review-response
title: composeOf duplicates unreadOf
finding: buttons.go:146-159 is unreadOf (validate.go:467-479) without the persist branch. Extract a strictBool(n *yaml.Node) (bool, problem string) helper that both call, so the two spellings of 'strict YAML boolean' cannot drift.
severity: nit
resolution: "addressed\tExtracted strictBool(n) in validate.go. composeOf and unreadOf both call it."
status: addressed
---
