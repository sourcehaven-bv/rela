---
id: RR-IWAGB8
type: review-response
title: AC 1 and AC 13 cannot add the example to valid.yaml verbatim
finding: 'internal/realtimecfg/testdata/valid.yaml already declares channel type incident-room with other message types, roles and a trigger (breach-escalated) that posts into it. testdata/schema.yaml has no risk-level property type, so `type: risk-level` and the guard `message.risk != ''none''` do not load. A verbatim paste is either a duplicate mapping key or a different type. Use a new channel type name (e.g. breach-response), declare risk-level as an enum in testdata/schema.yaml or use an inline enum, use final: true per state, and make GUIDE-realtime.md show exactly that block, so AC 13 still pins the docs example.'
severity: significant
resolution: 'Plan updated: test data uses a new breach-response type in valid.yaml with final: true, adds a risk-level enum to testdata/schema.yaml and updates TestParse_Golden; the guide shows the same block.'
status: addressed
---
