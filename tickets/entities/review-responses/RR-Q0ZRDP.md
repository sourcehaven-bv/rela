---
id: RR-Q0ZRDP
type: review-response
title: Type-mismatch 400 reveals entities of a type the caller cannot read
finding: '[security] The row gate checks the id against the document type. Under an AllowAll grant on that type it passes for any id; the face re-check treats a DenyAll type as every face allowed; so the 400 names the real type of an entity the caller may not read (existence and type oracle).'
severity: significant
resolution: A row of another type now clears getVisibleRef against its own type before the 400 names it; otherwise the uniform 404. Pinned by the type-mismatch subtests of TestAnchoredDocument_FaceGate (verified failing without the check).
status: addressed
---
