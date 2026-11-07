---
id: RR-BDETJJG
type: review-response
title: 'Design: arch-lint does not allow reconcile in the stores'
finding: pgstore and sqlitestore may not import internal/reconcile.
severity: significant
resolution: 'Plan changed: .go-arch-lint.yml gains reconcile for pgstore and sqlitestore, with the reason.'
status: addressed
---
