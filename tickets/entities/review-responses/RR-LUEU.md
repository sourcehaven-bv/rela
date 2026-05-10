---
id: RR-LUEU
type: review-response
title: mutateState republishes identical snapshot on save failure
finding: 'handlers_theme.go PUT path: when saveUserLogo fails, the mutator returns without modifying the snapshot copy, but mutateState still allocates and republishes. Cost is negligible (~96 bytes copied) so leave it, but add a one-liner comment so a future reader doesn''t try to ''optimize'' it.'
severity: nit
resolution: Added a comment inside the failing branch of the mutateState mutator in handleAPIPutThemeLogo explaining that the bytewise-identical republish is intentional and shouldn't be 'optimized'.
status: addressed
---
