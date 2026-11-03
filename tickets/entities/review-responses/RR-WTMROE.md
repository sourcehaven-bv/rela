---
id: RR-WTMROE
type: review-response
title: Stores do not import internal/markdown
finding: Only fsstore imports markdown; arch-lint gives pgstore, sqlitestore and memstore no markdown or goldmark edge.
severity: significant
resolution: New leaf package internal/mentions (goldmark + entity only). Arch-lint edges added for the four stores and dataentry, with justifying comments.
status: addressed
---
