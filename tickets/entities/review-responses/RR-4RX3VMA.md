---
id: RR-4RX3VMA
type: review-response
title: Duplicate flow, state and timer names load silently
finding: 'flows:, states: and after: decode into yaml.Node, which keeps repeated keys. A duplicated state left a ghost state the engine would run.'
severity: significant
resolution: 'mapping() refuses a repeated key with ''is declared twice''. Tests: duplicate flow, state and timer cases in TestParse_Problems.'
status: addressed
---
