---
id: RR-HQMI46V
type: review-response
title: Update and Delete must be single-statement CAS
finding: Select then update under READ COMMITTED loses updates.
severity: nit
resolution: Single UPDATE/DELETE with WHERE id AND rev; a re-read only tells ErrNotFound from ErrConflict.
status: addressed
---
