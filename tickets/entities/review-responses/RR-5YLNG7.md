---
id: RR-5YLNG7
type: review-response
title: Free-edge resolution rebases the marker onto the file's projection, dropping store divergence
finding: 'Taking a compatible free edge sets the walk position to the file''s to_projection; advanceMarker then records a projection that does not describe the store''s actual divergence (confirmed: an extra additive property disappears from the marker). Review feared the drift ledger could lose entries when a file re-declares a dropped property.'
severity: significant
resolution: 'Analysis (documented in resolve.go''s godoc, commit bddc13f3): the rebase is safe by construction. Ledger pruning runs against the LIVE schema, never the marker — if live re-declares the property, the data is genuinely no longer orphaned and pruning is correct; if it doesn''t, the next gate evaluation re-classifies the marker↔live diff and re-adopts (additive) or re-ledgers (drift). The only consequence is a restarted GC grace clock, which fails toward RETENTION, never toward deletion. `migrate data --apply` already ends with a gate re-evaluation, so the marker converges immediately.'
status: addressed
---
