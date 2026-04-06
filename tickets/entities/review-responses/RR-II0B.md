---
id: RR-II0B
type: review-response
title: daily-parse regex fragile for real-world markdown
finding: Pattern requires exact '- [ ] ' format. Won't match asterisk lists or double spaces. Inline markdown in text becomes part of task title. Fine for v1 but should strip formatting.
severity: significant
reason: Needs proper GFM task list support in rela's markdown package. Will create separate ticket for goldmark tasklist extension. Current regex is adequate for v1.
status: deferred
---
