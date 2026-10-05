---
id: RR-BQIO69
type: review-response
title: readableFaceOf passed a zero visibility.World
finding: readableFaceOf called refIn with visibility.World{}, the only production zero World left after an unset world became invalid (security review).
severity: minor
resolution: 'It now passes defaultWorldHandle().visibility(). The request world is not used on purpose: the world-absent page must answer the same under a denied world as under an empty one (TestEntityView_DeniedWorldIsIndistinguishableFromAnEmptyOne).'
status: addressed
---
