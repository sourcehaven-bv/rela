---
id: RR-GOCII8
type: review-response
title: config:edit is admin-equivalent and must be an explicit grant
finding: Declarative automations are confused deputies (cascades authorize as the originating principal, autocascade/host.go:29); migrations bypass the ACL; usage counts read the raw store.
severity: significant
resolution: 'Plan changed: docs state config:edit is admin-equivalent; ''*'' does not satisfy it (explicit grant only) and client baselines do not inherit it; migrations and usage counts additionally require AllowAll read of the affected type, visibility of the property and a write grant, else 403 naming the type.'
status: addressed
---
