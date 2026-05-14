---
id: RR-1DP3
type: review-response
title: 'Cranky #3: Workspace godoc and query.go reference removed features'
finding: Workspace godoc still said it owns 'automation engine'; query.go GetEntity comment claimed it satisfies autocascade.Host. Stale after the flip.
severity: significant
resolution: Updated package godoc + Workspace type doc to reflect entitymanager.Manager ownership and transitional shim status. Rewrote query.go's GetEntity comment without the host claim.
status: addressed
---
