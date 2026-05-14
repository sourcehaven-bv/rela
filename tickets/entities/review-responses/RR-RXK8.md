---
id: RR-RXK8
type: review-response
title: Stranded blank lines in EntityDetail.vue
finding: EntityDetail.vue has stranded blank lines at the locations where deleted blocks used to sit (template ~lines 673-674, 689-691, and ~1224 before </style>).
severity: minor
resolution: 'Removed three stranded blank lines in EntityDetail.vue: between </div> and </section> in the table-section block, between </div> and </div> at the end of the entity-detail wrapper, and the trailing blank line before </style>.'
status: addressed
---
