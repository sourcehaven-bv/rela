---
id: RR-L3OM2E
type: review-response
title: Raw store satisfies ingest.Reader
finding: Raw store satisfies ingest.Reader
severity: significant
resolution: ingest.Reader now includes GetAddress, which store.Store lacks. TestReader_RawStoreDoesNotSatisfy pins it.
status: addressed
---
