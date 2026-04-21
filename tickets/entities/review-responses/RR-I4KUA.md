---
id: RR-I4KUA
type: review-response
title: Legend 'except' clause may list the source as an exception
finding: '`formatTargets` takes `total = len(entityNames)`. For self-excluding relations (source not in its own target list), the complement size is `total - 1`, which triggers the ''any entity except Src'' branch — listing the source itself as an exception. Semantic noise. Fix: when the caller knows which entity is the source, pass `effectiveTotal` that excludes self.'
severity: minor
resolution: 'renderLegendNode now computes effTotal per entry: if source is not in its own target list, effTotal = total - 1 and formatTargets receives (source, srcInTargets=false) so the complement iteration skips self. Added TestFormatTargets/src_excluded_from_complement case: with source=''b'' (not in targets), 4 targets out of effectiveTotal=4 → ''any entity'', no ''except'' list.'
status: addressed
---
