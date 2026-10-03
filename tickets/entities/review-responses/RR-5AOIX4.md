---
id: RR-5AOIX4
type: review-response
title: Alias write grants on faced types load clean but grant nothing
finding: GrantsVerbOnState compares grant type names literally. An alias entry such as pol@draft for a faced type policy passed the new load check but granted nothing; the test asserted it was valid and the guide said the rule applies to aliases.
severity: significant
resolution: facedWriteGrantErrors now refuses an alias entry on a faced type (bare or alias@face) and names the canonical form (policy@draft). namesAnyFace counts only canonical face grants. Test and GUIDE-acl-security wording updated; new unit and appbuild tests pin the refusal.
status: addressed
---
