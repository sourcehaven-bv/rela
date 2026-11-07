---
id: RR-2BV7V1O
type: review-response
title: View-hash wrapper belongs in canonical
finding: reconcile.EntityViewHash would only rename canonical.Sum*; an ETag importing a sync package is backwards.
severity: minor
resolution: canonical gains Digest with SumEntity and SumRelation; HashEntity and HashRelation are their hex form. reconcile keeps Element and Fingerprint and Range and takes a canonical.Digest. dataentry builds the entity view with one helper and both ETags hash the view with canonical.Sum*.
status: addressed
---
