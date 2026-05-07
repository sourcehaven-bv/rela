---
id: RR-P4AST
type: review-response
title: Re-sending unchanged relation should be a true no-op, not a rewrite
finding: |-
    Plan: 'Re-sending an unchanged relation: no-op write — file rewritten with identical content. Slight timestamp churn but acceptable.' Not acceptable — this ticket exists for auto-save, which will re-PATCH the same data on every form interaction. Every PATCH rewriting every relation file means: file watcher fires on every save, SSE broker fires on every save, mtimes change, git diff blows up with no actual content change, disk wear is real on SSDs over time.

    Fix: diff classifier produces four buckets — keep (target+meta deep-equal), add, remove, update-meta. Keep edges are NOT staged. Transaction only touches disk for actual changes. Add AC: 'PATCH with relations exactly matching current state writes zero relation files (verify via repo write-counter).' Also: if keep is the only outcome AND properties unchanged, return 200 and DON'T broadcast entity:updated. Auto-save will hit this constantly.
severity: significant
resolution: 'Decision #11 + ACs #17, #18: diff classifier''s ''keep'' bucket suppresses no-op writes. PATCH where everything matches current state writes zero relation files. No-op PATCH does NOT fire entity:updated SSE event. Critical for auto-save which re-PATCHes constantly.'
status: addressed
---
