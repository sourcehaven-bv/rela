---
id: RR-1PCQ42
type: review-response
title: Comment count is an unfiltered aggregate and an existence oracle
finding: 'The plan''s UI shows a per-property comment affordance, which in practice means a count badge, and the panel lists comments — but the plan says only ''decide whether the section needs a cap''. docs/acl-security.md:638 states the rule directly: ''No count from an unfiltered source. Any new aggregate (badge, dashboard card, export count) must derive from the gated set.'' CLAUDE.md''s gantt rule is the worked precedent: gate BEFORE you fold, and compute caps/truncated on the filtered set, because a pre-filter count is an existence oracle. Two concrete requirements the plan currently omits: (1) any comment count must be computed after the comment:read verdict, never from a raw store count; (2) if a cap is applied, ''truncated'' must be post-filter (TestGantt_TruncatedIsPostFilter is the model). Also unaddressed: whether a principal with comment:read but WITHOUT read on the target can infer the target''s existence from a 403-vs-404 divergence — AC7 covers the read floor but not the count surface.'
severity: significant
resolution: 'Plan now has a ''Counts are post-gate'' section: any comment count (per-property badge, panel header) is computed after the comment:read verdict, never from a raw store count, per docs/acl-security.md:638. If a cap is applied, truncated is post-filter following TestGantt_TruncatedIsPostFilter. AC13 pins it.'
status: addressed
---
