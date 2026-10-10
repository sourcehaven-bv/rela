---
id: RR-GOCII8
type: review-response
title: config:edit is admin-equivalent and must be an explicit grant
finding: Declarative automations are confused deputies (cascades authorize as the originating principal, autocascade/host.go:29); migrations bypass the ACL; usage counts read the raw store.
severity: significant
resolution: 'docs state config:edit is admin-equivalent; ''*'' does not satisfy it (explicit grant only) and client baselines do not inherit it. Corrected after the CISO review on #1792: migrations and usage counts do NOT check per-type read/visibility/write grants. A config:edit holder rewrites the schema and so can grant itself any access; a per-type check would not bound that. Usage counts return numbers only and are audited; migrations write config-edit and data-migration audit records.'
status: addressed
---
