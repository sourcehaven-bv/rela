---
id: RR-8UHVOJA
type: review-response
title: 'Code: sqlite heal takes the write lock every tick'
finding: fillElements runs in s.Tx, so every sweep tick took the write lock even with nothing to heal.
severity: minor
resolution: healElements first probes SELECT EXISTS(... WHERE element_hash IS NULL) per table outside a transaction, served by the partial elementless index, and skips the fill when nothing is unhashed.
status: addressed
---
