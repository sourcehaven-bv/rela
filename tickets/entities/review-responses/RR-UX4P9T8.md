---
id: RR-UX4P9T8
type: review-response
title: "keptWidth repeats the compressed cell rule"
finding: "keptWidth in columnSizing.ts rebuilds the width the compressed cell CSS gives, so the two can drift."
severity: nit
reason: "The width uses a container unit and a token, which only CSS can evaluate; both places name each other in their comments."
status: wont-fix
---
