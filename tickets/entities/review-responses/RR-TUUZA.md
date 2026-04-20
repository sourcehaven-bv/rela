---
id: RR-TUUZA
type: review-response
title: reseal_sentinel.go and reseal.go still reference xdg-state and call NewLocalState — plan missed them
finding: reseal_sentinel.go:92-99 calls NewLocalState(repoID) directly; reseal.go:264-270 has a hard-coded <xdg-state> placeholder error message. Neither is listed in the plan's file-modification list and both break with the new NewLocalState(svc) signature.
severity: significant
resolution: 'Added to Files to modify list. reseal_sentinel.go: adopt new signature, take userstate.Service. reseal.go: update error-fallback placeholder to match new layout (or better: use service.Path for diagnostic messages).'
status: addressed
---
