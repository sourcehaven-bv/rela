---
id: RR-TFOEDA
type: review-response
title: internal/mcp 5.7pp from default floor with no visible marker
finding: Raising the default floor to 50 silently put mcp (55.7%, large and churny) closest to failing, with nothing in the config documenting the proximity.
severity: significant
resolution: Explicit ^internal/mcp$ override at 50 with a comment documenting the ~56% proximity, per the file's stated visible-not-silent philosophy.
status: addressed
---
