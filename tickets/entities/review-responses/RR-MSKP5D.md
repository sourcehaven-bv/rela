---
id: RR-MSKP5D
type: review-response
title: analyze owning swallows store errors
finding: An iterator error was skipped, so the check could report clean data after reading nothing.
severity: minor
resolution: CheckOwning returns the error and the CLI fails. TestCheckOwning_StoreErrorFailsLoudly.
status: addressed
---
