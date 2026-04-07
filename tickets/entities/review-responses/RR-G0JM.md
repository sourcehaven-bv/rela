---
id: RR-G0JM
type: review-response
title: 'N2: truncate sliced UTF-8 mid-rune, producing invalid output'
finding: truncate(s, 200) used s[:n] which can split a multibyte rune in half if the attacker sends a UTF-8 Origin. Logs would contain garbage prefixes.
severity: nit
resolution: Rewrote truncate to operate on []rune and use the U+2026 horizontal ellipsis as suffix instead of three dots. Rune-aware so multibyte input is never split.
status: addressed
---
