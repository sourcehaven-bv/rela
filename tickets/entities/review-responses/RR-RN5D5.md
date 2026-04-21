---
id: RR-RN5D5
type: review-response
title: Consider typed problem+json error responses for structured client handling
finding: Callers match on free-form `detail` prose. A dedicated `type` URL like https://rela.dev/errors/invalid-id-prefix plus a structured allowed_prefixes extension field would let the UI show prefix-picker inline and let tests assert on error type rather than scraping English.
severity: nit
reason: Same destination as RR-O1UMW. Tracking as a single follow-up to design typed RFC-7807 error codes across the data-entry API; once that lands, the e2e specs (RR-O1UMW), the validator return type (RR-ODPMN), and the EntityDef.MatchesID consolidation (RR-8T3VM) can all switch to the structured shape in a coordinated change.
status: deferred
---
