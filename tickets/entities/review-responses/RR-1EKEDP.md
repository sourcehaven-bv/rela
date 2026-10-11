---
id: RR-1EKEDP
type: review-response
title: Mine lists entities an attenuated client cannot read
finding: 'Security review: Identity ignores principal type and scopes so a browser session and an agent token share leases; Mine returns them without a read check, disclosing ids, status and label across the client ceiling.'
severity: significant
resolution: 'Fixed in f9bddab4 (Lease service: review fixes) with a test; see the commit.'
status: addressed
---
