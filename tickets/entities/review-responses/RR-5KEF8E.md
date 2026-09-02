---
id: RR-5KEF8E
type: review-response
title: 'Roll-up must fold post-filter: folding raw entities leaks hidden dates into a visible parent''s span'
finding: 'The plan says roll-up ''folds only visible descendants'', but does not pin the ORDER against the codebase''s established traverse pattern — which does the opposite. internal/dataentry/views.go:47-68 traverses RAW store entities on purpose (a rule''s where: may name a hidden property, edges walked by id) and row-gates/redacts only on the way OUT. If the gantt copies that shape and folds during traversal, a visible parent''s rolled span is computed from hidden children''s dates. Filtering afterwards does not undo it: the parent''s start/end silently encode a hidden entity''s dates, which is a VALUE disclosure, not the one-bit membership channel views.go explicitly accepts as residual. A principal could binary-search a hidden child''s due date by watching the parent''s rolled end. Worse, breach.before/after flags are derived from the same arithmetic, so they leak too. The fold MUST run strictly after row-gating, on the already-filtered node set. This also compounds RR-Y7MINP (counts/truncation flag). Needs an explicit test: same tree, two principals, assert the restricted principal''s parent span is computed WITHOUT the hidden child — i.e. it differs from the privileged principal''s.'
severity: critical
resolution: 'Plan updated: the Approach section now states ''ORDER IS A SECURITY PROPERTY — gate, then fold'' as an explicit three-step sequence (row-gate → post-order fold → cap/truncated), contrasted against the views.go:47-68 traverse-raw-then-gate pattern with the reason it is unsafe for an aggregate. Security Considerations carries the invariant plus the per-principal no-cross-principal-caching rule. AC7 was rewritten from a generic ''hidden entities never leak'' into the concrete two-principal differential test: a hidden child that is the max-end descendant must make the restricted principal''s parent rolled.end differ from the privileged principal''s. Also flagged for a CLAUDE.md entry at implementation time, since this invariant is easily reversed by a later refactor.'
status: addressed
---

## Finding

The plan asserts roll-up "folds only visible descendants", but does not state
**where in the pipeline** the fold runs — and the pattern it would naturally be
copied from does the opposite.

`internal/dataentry/views.go:47-68` documents the existing traverse contract:

> Traversal above runs on raw store entities on purpose: a rule's `where:`
> filter may reference a hidden property, and edges are walked by id —
> redacting mid-traversal would break both. Redacting here, once, gives every
> section builder already-redacted entities.

That is correct for flat collections. It is **unsafe for a fold.**

## Why a fold is different

Filtering after the fact removes hidden *rows*. It cannot remove their
*contribution to an aggregate already computed from them*.

If the gantt traverses raw and folds on the way up:

- A visible parent's `rolled.end` = max(end of ALL descendants, hidden
included).
- The hidden child's date is now embedded in a field the principal may read.
- This is a **value disclosure**, categorically worse than the one-bit
membership inference `views.go` accepts as residual.
- An attacker can narrow a hidden child's dates by observing how the parent's
span moves as data changes.
- `breach.before` / `breach.after` derive from the same arithmetic, so they
leak on the same channel — and a boolean that flips on a hidden entity's dates
is a clean oracle.

## Resolution

State explicitly, as an invariant with a comment at the fold site:

**gate → then fold.** The post-order roll-up runs over the already-row-gated
node set. A subtree dropped for visibility contributes nothing to any ancestor's
`rolled` span, `breach` flags, counts, or truncation flag.

This costs the `where:`-on-hidden-properties capability that `views.go`
preserves by traversing raw — accept that. A gantt source's `where:` should be
documented as evaluated post-gate.

## Test

Two principals over one fixture where a hidden child is the max-end descendant.
Assert the restricted principal's parent `rolled.end` equals the max over
*visible* children only — i.e. it genuinely differs from the privileged
principal's — and that `breach.after` differs accordingly.
