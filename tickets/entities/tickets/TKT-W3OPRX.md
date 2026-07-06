---
id: TKT-W3OPRX
type: ticket
title: Consolidate remaining triplicated markdown-body CSS into shared .md-body stylesheet
kind: refactor
priority: low
effort: s
status: done
---

# Consolidate remaining triplicated markdown-body CSS into shared .md-body stylesheet

**RESOLVED / SUBSUMED by TKT-YYZRGW.** That ticket's scope grew from tables-only
into a full markdown style-fixes PR that moved *all* shared markdown element CSS
(headings, lists, code, blockquote, hr, img, links, tables, kbd) into
`frontend/src/styles/markdown-content.css` under `.md-body`, removed the
per-component copies, re-skinned the EasyMDE preview, and added a mirror-sync
test. Nothing left to do here.

---

(Original) The three components each carried near-identical `:deep(...)`
markdown element blocks that had drifted (h1 28px vs 24px). Consolidating them
into the shared `.md-body` sheet was the follow-up leverage win — now done as
part of TKT-YYZRGW.
