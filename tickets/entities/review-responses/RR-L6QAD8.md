---
id: RR-L6QAD8
type: review-response
title: '`compose: "yes"` is not refused; a plain bool field accepts YAML 1.1 spellings'
finding: 'The plan adds `Compose bool yaml:"compose"` (Approach 2) and lists the negative test "`compose: \"yes\"` (a string): refused by the decoder as not a boolean". yaml.v3 decodes the strings y/yes/on/n/no/off into a typed bool, quoted or not (gopkg.in/yaml.v3 decode.go:678-693). So `compose: yes` and `compose: "yes"` both load as true, and the negative test will fail or be written to pass wrongly. The repo''s convention for new boolean keys refuses these spellings through a node-based reader: `rawUnread` with `yamlscalar.PlainYAML11Bool` (internal/realtimecfg/raw.go:253-259, internal/realtimecfg/validate.go:466-479) and `confirmOf` (internal/realtimecfg/buttonoptions.go:118-121). Use a `rawCompose` node type with the same check, and test `compose: yes`, `compose: "yes"` and `compose: 1`.'
severity: significant
resolution: 'ADDRESSED compose is a rawCompose node read by composeOf with yamlscalar.PlainYAML11Bool, as unread and confirm are. compose: yes, "yes", on and 1 are refused and tested (AC 1, Approach 2).'
status: addressed
---
