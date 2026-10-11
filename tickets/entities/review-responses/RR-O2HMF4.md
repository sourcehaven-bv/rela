---
id: RR-O2HMF4
type: review-response
title: MCP validated the address later than HTTP
finding: readableType touched the store before the safe-segment check, and a read-only instance answered not_found for an absent id where HTTP answers read-only
severity: minor
resolution: 'Fixed: address() rejects an unsafe address before any read, and the four write methods refuse a read-only instance before resolving the address (asserted in TestMCPComments_ReadOnly with an absent id). storedTypeOf still reads a store error as a miss; that is the shared helper every by-id write path uses (write_handler), so it is not changed here.'
status: addressed
---
