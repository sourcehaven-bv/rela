---
id: RR-IVGR4H
type: review-response
title: channels_per_entity counts all types and is check-then-act
finding: service.go lists every type on the anchor, contrary to the per-type limit and its error text; concurrent creates exceed a configured limit lower than the store's built-in one.
severity: significant
status: open
---
