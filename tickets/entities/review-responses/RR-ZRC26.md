---
id: RR-ZRC26
type: review-response
title: 'display: cards / content / table have different shapes; plan must specify per-mode'
finding: list = <a> with no href, broken click (needs both href + click fix). cards/content = <article>, no anchor, broken click only (needs click fix; promoting to anchor is a CSS change out of scope unless we want it). table = <a> with server-resolved href + broken click (cell.link is already correct; only the click handler is broken). Plan says 'update all callers' without distinguishing. Spell out the per-mode change so the implementer doesn't convert <article> to <a> and break the cards-grid CSS.
severity: significant
resolution: Per-display-mode plan spelled out in fix planning section. List = <a> + href + click. Cards/content = <article> click handler only. Table = preserve server cell.link as href, change click handler. No <article> -> <a> conversion.
status: addressed
---
