---
id: RR-F7AZKE
type: review-response
title: Face delete ignores cascade and swallows count errors
finding: 'DeleteEntityFace always removes the edges tailed at the face. CLI and MCP refused a face delete with edges only through a best-effort pre-count: the CLI discarded the CountRelations error, MCP counted only visible edges (so edges to hidden targets went without cascade), and Lua never refused. The same address behaved differently per surface. MCP part raised by the security reviewer as well.'
severity: significant
resolution: 'Defined one rule on every surface: cascade guards the family delete only; a face''s tailed edges are its content and go with it, as in data-entry and Lua. CLI and MCP no longer refuse a face delete, report the removed edges, and the CLI surfaces a count error. Tool description, CLI help and docs/lua-scripting.md say so. Tests: TestDeleteCmd_CascadeGuardsTheFamilyOnly, TestHandleDeleteEntity_FacedEntity.'
status: addressed
---
