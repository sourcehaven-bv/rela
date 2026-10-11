---
id: RR-8LNWD1
type: review-response
title: No behavior test that actions provision
finding: 'The provision invariant became an AST check through the delegated op, and no test drove an action with unmatched_principal: provision on either transport.'
severity: significant
resolution: 'Added TestProvision_ActionProvisionsOnBothTransports: an unmatched verified caller runs an action over HTTP and over the MCP mount, and exactly one person stub is created.'
status: addressed
---
