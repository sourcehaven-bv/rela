---
id: RR-SZCN3ZH
type: review-response
title: Default memory mail sender cannot be inspected
finding: The default MemorySender is not reachable by the caller.
severity: minor
resolution: NewInMemory godoc says default mail is discarded and points to WithMailTransport for inspection.
status: addressed
---
