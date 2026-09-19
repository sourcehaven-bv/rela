---
id: RR-LDPV50
type: review-response
title: A duplicate:-only entity_views block fails config load today
finding: 'validate.go:943-947 rejects any entity_views entry whose detail_view is empty: ''entity_views[%q]: detail_view is empty (omit the entry instead)''. An operator writing only entity_views.invoice.duplicate.properties gets that error, whose advice is actively wrong once duplicate: exists there. Separately, the plan''s claim that the SPA needs no new plumbing is only half true: responses.go:600 reuses dataentryconfig.EntityViewConfig so a new Go field reaches the wire for free, but a repo-wide grep finds zero hits for entity_views/entityViews anywhere in frontend/src, and DetailView has no reader outside config/validation/tests. The SPA needs the type, the store field and the reader.'
severity: significant
resolution: Added AC19c (a duplicate:-only entity_views entry must load) and scoped relaxing validateEntityViews. Corrected the effort reasoning: the Go wire is free, the SPA plumbing is not.
status: addressed
---

## Resolution required

1. Relax `validateEntityViews` to "at least one of `detail_view` / `duplicate`
must be present", keeping the existing error for a genuinely empty entry. Add an
AC — AC10 covers invalid `duplicate:` blocks but not this interaction.
2. Correct the effort reasoning: the wire is free, the SPA side is not. Not hard,
but not zero, and the estimate leaned on it being zero.
