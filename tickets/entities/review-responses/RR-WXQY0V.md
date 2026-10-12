---
id: RR-WXQY0V
type: review-response
title: pg migration number must stay gap-free on the stack
finding: pgstore.Migrate keeps a single version; 0026 on a tree without 0025 would skip a later 0025. On the develop-merged stack 0024 is mention targets and 0025 leases, so 0026 must land only there.
severity: significant
resolution: 0026 header explains the number and the renumber rule; stack has 0024 mention targets and 0025 leases, so no gap.
status: addressed
---
