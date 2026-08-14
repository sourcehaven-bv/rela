---
id: RR-DR-SYMLINK
type: review-response
title: The nested OpenRoot('custom') is security-critical, not an incidental simplification, and has no
  test
finding: 'Independent probe confirmed the containment chain holds against every vector tried (in-project
  symlink, absolute symlink, symlinked updir, Windows separators, NUL, URL-encoded dots, macOS resource
  fork). But the reviewer isolated WHY: a symlink inside custom/ pointing at ../private/keys.txt stays
  INSIDE the project root, so a single os.OpenRoot(projectRoot) + Open(''custom/''+rel) would have FOLLOWED
  it and served the file. Only the second, narrower root stops it. The plan describes the nested root
  as ''copying openAppEntry''s chain with one OpenRoot level'' - framing a load-bearing security step
  as an incidental structural detail. A future ''simplification'' to one root silently re-opens an in-project
  symlink read. The probe vector list also has no in-project symlink case, only ../secret.txt spellings.'
severity: significant
status: addressed
resolution: Folded into TKT-IWMETE and PLAN-6VVJJZ before implementation. Name the property in a code
  comment at the nesting site, and add a test with a symlink inside custom/ pointing to a project file
  OUTSIDE custom/ (not to /etc/passwd - the in-project case is the one a single root would miss). Reviewer
  also suggests retrofitting the same test into TestOpenAppEntry_Traversal, which protects the pattern
  being copied; that is a separate one-line addition, noted but out of scope here.
---

Raised by `/design-review` of TKT-IWMETE before implementation.
