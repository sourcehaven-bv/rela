---
id: RR-YP8R
type: review-response
title: 'Scope creep risk: edit mode adopting the live channel should stay out'
finding: 'Plan correctly scopes edit-mode adoption OUT but says ''build the server side so it can.'' Watch that ''build so it can'' does not become speculative generality (CLAUDE.md: don''t design for hypothetical future requirements). The dry-run endpoint should be exactly what create needs; if edit later wants it, edit''s PR adds it. Keep the server change minimal — one early-return, verdict-only — not a generalized validation framework. Reframe the OUT item as ''edit may reuse the same dry-run endpoint later; no extra abstraction now.'''
severity: nit
resolution: 'Plan reframed: the dry-run is exactly what create needs (one early-return, verdict-only). Edit may reuse the SAME endpoint later in its own PR; no abstraction built ahead of need. Out-of-scope item updated accordingly.'
status: addressed
---
