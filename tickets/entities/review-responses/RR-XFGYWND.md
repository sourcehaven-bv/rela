---
id: RR-XFGYWND
type: review-response
title: The local-role prime in primePage is untested on the appbuild path
finding: 'Code review: In dataentry the redactor primes local roles itself, so removing the PrimeLocalRoles call in primePage fails no test; the appbuild PolicyRedactor depends on it.'
severity: significant
resolution: TestScriptReader_PushdownPrimesLocalRolesUnderPolicyRedactor lists 10 and 50 tickets through ScriptReader over PolicyReader and PolicyRedactor with a role relation and visible grants. Without the call it reads 11 and 51.
status: addressed
---
