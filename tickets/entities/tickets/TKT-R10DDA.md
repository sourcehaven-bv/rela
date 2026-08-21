---
id: TKT-R10DDA
type: ticket
title: 'Attachment face-links: state-scoped membership over shared blobs'
kind: enhancement
priority: high
effort: m
status: backlog
---

Design doc §6.3. Prevents the leak: a screenshot uploaded to a draft, never
promoted, fetchable by a published-only reader because attachments key on the
entity.

Attachment membership belongs to the face: per-state link sets over shared
(refcounted/content-addressed) blobs; the copy kernel carries the link set;
download gate = some face the principal may read (in a world they hold) links
it. Explicit link rows, not body parsing. Migration: existing attachment rows
become links from the default state. Acceptable interim restriction if Step 1
needs trimming: forbid attachments on non-default states — never ship
entity-scoped gating alongside states.
