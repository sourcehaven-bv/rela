---
id: RR-A0PY0M
type: review-response
title: Two meanings of hidden in one block
finding: 'In the same channels block, a field''s hidden: true means never shown and never sent (channels.go:22-25), while display: hidden on a message type means shown behind a toggle. Elsewhere in data-entry.yaml, display: chooses a rendering mode (config.go:1499, 1956). The name is acceptable, but docs/data-entry.md should contrast the two hiddens in one sentence.'
severity: nit
resolution: 'docs/data-entry.md gets one sentence contrasting a field''s hidden: true (never sent) with display: hidden on a message type (sent, shown behind the toggle).'
status: addressed
---
