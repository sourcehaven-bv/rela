---
id: RR-Q0PBSK
type: review-response
title: Force-live erasure lasts only until the next capture
finding: An edit, rename or delete after a force-live purge records the live row again; docs said only prefer redacting first (security review, existing behaviour).
severity: minor
resolution: The --force-live flag doc now says the tombstone stops only a re-capture of the same content and that lasting erasure needs the live value redacted or the row deleted first.
status: addressed
---
