---
id: RR-JTBTUU
type: review-response
title: 'Nits: hex loop concat, empty-string test, dead double-process guard'
finding: (8) encodePlantUMLHex string-concats in a loop — use Array.from(...).join(''). (9) add a test asserting serverURL='' (empty string, the actual YAML-disabled value) is a no-op. (7) the Form-2 'won't happen' guard comment cargo-cults mermaid — keep but justify, or drop.
severity: nit
resolution: (8) encodePlantUMLHex now uses Array.from(bytes, ...).join(''). (9) added empty-string ('') no-op test. (7) Form-2 guard comment rewritten to explain the invariant (goldmark=Form1, server-rewrite=Form2, don't co-occur) rather than 'won't happen'.
status: addressed
---
