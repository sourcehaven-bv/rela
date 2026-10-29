---
id: RR-W1ACQL
type: review-response
title: Variation-selector refusal breaks common LLM emoji and is undocumented
finding: 'texthazard.go:28 refuses every variation selector, so options such as ''✔️ Done'', ''⚠️ Risky'' or ''❤️'' (which agents emit all the time) get 400 ''holds U+FE0F''. docs/realtime.md:255 lists control, line-break and invisible characters, but not variation selectors, so an agent author cannot predict the refusal. Fix: preferably strip U+FE0E and U+FE0F during normalization (they cannot create a duplicate after stripping), keeping the refusal for the other selectors. At minimum, name variation selectors in docs/realtime.md and in the MCP post_message description.'
severity: minor
resolution: 'Allowed as safe: normalizeListItem drops U+FE0E and U+FE0F (texthazard.DropStyleSelectors) before trim and NFC, so two items that differ only by a style selector are equal and the duplicate check catches them. Every other variation selector is still refused. Added tests for drop, the duplicate and a refused U+FE00. Documented in GUIDE-realtime, GUIDE-mcp-server and the MCP post_message description (golden regenerated).'
status: addressed
---
