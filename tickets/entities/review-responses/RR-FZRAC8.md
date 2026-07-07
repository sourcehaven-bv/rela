---
id: RR-FZRAC8
type: review-response
title: rela.calendar module is pure but the feed still runs a full runtime — keep the store-read curation in the script, confirm ReadDeps availability
finding: 'The plan says rela.calendar is a ''pure builder/renderer needing no ReadDeps'' and registers it in registerReadBindings — correct and good (matches markdown.go). But note the composition: the FEED SCRIPT still needs rela.list_entities / entity:prop / rela.url to curate events, which DO come from the runtime''s ReadDeps and the route catalogue (rela.url requires WithRouteCatalog per PLAN-3E5HR). Confirm the feed execution path wires the route catalogue (so rela.url resolves inside a feed script) and the ReadDeps store (so list_entities works). The rela.calendar module itself is pure, but the script that uses it is not — the plan''s file list should ensure the feed runtime is constructed with WithRouteCatalog and full ReadDeps, exactly as document rendering is. Otherwise rela.url in a feed script raises ''route catalogue not configured''.'
severity: minor
resolution: 'Plan updated (PLAN-6LOL0Z file list + §2): the feed runtime is constructed with the route catalogue (WithRouteCatalog) + full ReadDeps so rela.url and rela.list_entities resolve inside a feed script; the rela.calendar module itself stays pure. A WithFeedMode option exposes rela.feed.* to the script.'
status: addressed
---
