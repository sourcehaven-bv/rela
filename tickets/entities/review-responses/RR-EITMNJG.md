---
id: RR-EITMNJG
type: review-response
title: Prefix error message does not say the match ignores case
finding: The ValidateEntity message lists declared prefixes such as [nt-] without saying the match ignores case.
severity: nit
reason: The message is unchanged and already names the declared prefixes. Ids are case-insensitive throughout, so saying it in one message only would be inconsistent.
status: wont-fix
---
