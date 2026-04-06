---
id: RR-1LRP
type: review-response
title: Simple YAML parser may break on quoted strings and comments
finding: 'The plan proposes a simple line-by-line YAML parser for palette.yaml files. Rela writes YAML with quoted hex values (e.g. accent: "#6366f1") — the parser needs to handle both quoted and unquoted values. Also needs to handle inline comments (key: value # comment). Consider using a lightweight regex approach rather than true YAML parsing.'
severity: minor
resolution: YAML parser uses regex that strips optional quotes around hex values
status: addressed
---
