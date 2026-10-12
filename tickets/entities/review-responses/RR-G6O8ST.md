---
id: RR-G6O8ST
type: review-response
title: Separate leasestate package for one interface
finding: 'Architecture design review: the service is the store''s only consumer.'
severity: significant
resolution: 'Plan: one internal/lease package with kvlease, pglease, leasetest subpackages.'
status: addressed
---
