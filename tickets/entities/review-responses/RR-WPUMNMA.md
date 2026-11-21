---
id: RR-WPUMNMA
type: review-response
title: The watch source packs three states into one value
finding: undefined meant hold, null clear and a string set, which a well-meant cleanup could break.
severity: nit
resolution: The source is now an explicit { seen, language } object.
status: addressed
---
