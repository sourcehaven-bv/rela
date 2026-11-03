---
id: TKT-S81XJH
type: ticket
title: Template picker as dropdown with readable variant labels
kind: enhancement
priority: low
effort: s
started: "2026-10-12"
completed: "2026-10-12"
status: done
---

## Description

The create form listed entity template variants as a row of pill buttons. With
many variants (eight for one type in a real project) the row overflowed and
scrolled sideways. Variant labels also kept the file-name form, so
`corrective_action` showed as "Corrective_action".

The form now shows the variants in a labelled "Template" dropdown (`RlSelect`),
only in create mode and only when there is more than one template. Preselection
(`?template=`, `embeddedTemplate`, section `create.types.<type>.template`) and
switching behaviour are unchanged. Labels show `-` and `_` as spaces; the base
template stays "Default".
