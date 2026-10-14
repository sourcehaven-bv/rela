---
id: RR-EK9YLX
type: review-response
title: Provision move widens scope and adds a write dependency to access
finding: Moving all of maybeProvision brings stub creation and entitymanager into access, beyond the identity part the ticket names.
severity: significant
resolution: Stub creation stays in dataentry. access.Opener.Restamp does the re-resolve, matched-verified mark and ForPrincipal. A read-gate failure still returns the original ctx. access depends only on acl and principal.
status: addressed
---
