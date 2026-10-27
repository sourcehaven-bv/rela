---
id: RR-9DL1H4
type: review-response
title: World used for counts but dropped from links and refetch
finding: List cards counted in ?world= but the card and cell links opened the default world, and a world change left stale counts.
severity: significant
resolution: 'cardLink and the matrix cellLink carry world=; the view watches props.name and worldParam together. Test: counts in the page world, links into it and recounts when it changes.'
status: addressed
---
