---
id: RR-FUBK
type: review-response
title: Use range instead of C-style for loop for alignments
finding: for i := 0; i < len(table.Alignments); i++ should be for i, a := range table.Alignments — more idiomatic Go.
severity: nit
resolution: 'Changed to range-based iteration: for i, a := range table.Alignments.'
status: addressed
---
