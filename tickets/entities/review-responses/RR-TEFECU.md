---
id: RR-TEFECU
type: review-response
title: Table output sanitizes only the listed fields; author user, names and error text also reach the terminal
finding: Section 0 lists props, title, reason and ref titles as untrusted. But Author.User comes from IdP or proxy headers, and DenialError titles echo caller-supplied type names. Route every string field in channel_render.go through printable, not an allowlist of fields.
severity: minor
resolution: Every string written by channel_render.go goes through printable.
status: addressed
---
