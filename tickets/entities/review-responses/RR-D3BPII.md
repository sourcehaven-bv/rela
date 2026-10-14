---
id: RR-D3BPII
type: review-response
title: Classification view evaluates only the zero face
finding: internal/cli/classification_acl.go evaluates entity.New("" , t), the zero face. With face-scoped reads a block-less role contributes nothing there, so the worst-case report can claim fields are hidden that are visible on a face the role reads. Evaluate each declared face and take the union.
severity: minor
resolution: classificationViewFor evaluates each declared face of a type (or the unnamed face of a faceless type) and unions the visible fields, for entities and relation meta.
status: addressed
---
