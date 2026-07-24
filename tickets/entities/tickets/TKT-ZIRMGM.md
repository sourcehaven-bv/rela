---
id: TKT-ZIRMGM
type: ticket
title: 'Author-aware version capture: last_edited_by column + flush-on-author-change (precise per-version attribution)'
kind: enhancement
priority: high
effort: m
status: review
---

Follow-up to TKT-9INY0Y (pgstore content versioning). User idea, 2026-07-08.

## Field report (2026-07-24)

Confirmed broken-feeling in practice on an oauth2proxy-gated rela data-entry
deployment: the version-history UI for an entity (beleid · POLICY-016) shows
every version — including plain user edits — as `unknown · version-sweep`. The
proxy forwards a real authenticated user, so the write path HAS the principal;
it just never reaches the version rows because create/update capture happens in
the sweep. In practice this makes the history's "who" column useless for the
primary use case (who changed this policy?).

## Problem

In v1, sweep-captured create/update versions are attributed to a SYSTEM
principal (`{tool: "version-sweep"}`); the real editing principal is only
recoverable by fuzzy correlation against the audit log (accepted compromise
RR-3L2O7Y). Worse, the debounce collapses a burst of edits into one version
regardless of author — so if two different users edit an entity within the same
idle window, their changes merge into a single version attributed to neither
precisely.

## Idea

Two coordinated changes:

1. **Store `last_edited_by` (principal) on every entity write.** A new column on the live `entities` row (and the domain entity), set at the entitymanager write boundary from ctx. The sweep then reads it and stamps the version with the REAL author — no audit-log correlation needed. This removes the system-principal compromise entirely.

2. **Flush-on-author-change (author-aware debounce).** When a write arrives whose principal differs from the entity's current `last_edited_by` AND the entity has an un-snapshotted pending change (current content hash != latest version's content hash), synchronously capture a version of the PRE-EDIT state attributed to the PREVIOUS author first, then apply the new write and update last_edited_by. This preserves the debounce benefit *within* a single author while guaranteeing an author boundary always produces a distinct, correctly-attributed version. (Same semantics as collaborative-editor revision history segmenting by author.)

## Why it's better

- Precise per-version authorship on the entity row itself; deletes the "attribution via audit-log correlation" fallback.
- No two authors' edits ever silently merge into one mis-attributed version.
- Reuses machinery already built: the synchronous-capture path (delete/rename) and the content-hash "pending change" check the sweep already computes.

## Design notes / decisions for its own planning

- **Where the flush decision lives:** the WRITE PATH (entitymanager), not the sweep — deterministic, not sweep-timing-dependent. The sweep keeps handling the same-author debounce case.
- **Pending-change detection:** compare live content hash to latest version's content hash (cheap; sweep already does this).
- **Blast radius:** adds a column to the hot `entities` table + the domain `entity.Entity` + every backend's write. Decide whether `last_edited_by` is postgres-only (versioning is pg-only) or a general entity field. If general, it interacts with the canonical content hash (must be EXCLUDED from HashEntity, like UpdatedAt, or every write churns the hash).
- **Attribution trust:** same model as audit (principal from ctx); a forged principal under an untrusted proxy would forge last_edited_by too — document, don't re-solve.
- Consider whether this supersedes / simplifies the sweep's create/update attribution or replaces part of it.
