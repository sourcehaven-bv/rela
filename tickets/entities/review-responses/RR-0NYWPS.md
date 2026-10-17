---
id: RR-0NYWPS
type: review-response
title: Changing a property away from ref exposes stored entity ids
finding: render.go decides ref rendering from the current config; CheckReload compares only anchor and cardinality. Changing a ref property to string (or replacing a message type) renders stored ref ids as plain strings to every reader, including readers who cannot see the entity.
severity: significant
status: open
---
