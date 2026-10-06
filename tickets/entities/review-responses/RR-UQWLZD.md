---
id: RR-UQWLZD
type: review-response
title: Untrusted copy could delete secrets
finding: keychainSecrets.remove had no place check, so a copy carrying the document ID could delete the original's secrets.
severity: significant
resolution: remove takes the place and returns errNotTrusted like set; the settings page hides Remove while untrusted. Tested.
status: addressed
---
