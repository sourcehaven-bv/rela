---
id: RR-PVZ2SD
type: review-response
title: 'where: match ERROR silently dropped entities and narrowed ancestor roll-ups'
finding: 'filter.MatchAll errors (unparseable stored value under a comparison, e.g. planned_end: "soon") were folded into the non-match branch: the entity vanished with no signal and its dates vanished from every ancestor''s rolled span — the ''looks on schedule'' failure the view exists to prevent — while the adjacent ParseAll branch fails closed with a 500.'
severity: significant
resolution: 'The error branch is now distinct: the entity is still excluded (for a filter, exclusion is the closed direction; a value that cannot be compared cannot be trusted in the fold either) but the drop is logged via slog.Warn with entity id and error, and the choice is documented at the site. Pinned by TestGantt_WhereMatchErrorExcludes. Also added TestGantt_WhereOnHiddenPropertyMatchesNothing, turning the post-redact membership contract from prose into a test.'
status: addressed
---
