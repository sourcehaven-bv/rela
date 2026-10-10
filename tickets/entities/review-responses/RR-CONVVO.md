---
id: RR-CONVVO
type: review-response
title: Pile id collision is not retried
finding: kvpiles CreatePile does not check id uniqueness; pgpiles turns a primary-key collision into a 500.
severity: nit
resolution: kvpiles returns ErrIDTaken on an existing id and pgpiles maps a piles_pkey collision to it; the service retries up to 3 times. pilestest RunIDTests, TestService_CreateRetriesIDClash.
status: addressed
---
