---
id: RR-ZY04OK
type: review-response
title: Boot field refusal not re-run on reload
finding: QueryScopeTraversalFieldErrors runs only in prepare; after a live reload the per-request gate refuses instead, contrary to the docs.
severity: minor
resolution: Docs now say the startup refusal becomes a per-request refusal after a live reload of schema.yaml or acl.yaml. The per-request gate enforces the same rule.
status: addressed
---
