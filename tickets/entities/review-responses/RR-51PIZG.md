---
id: RR-51PIZG
type: review-response
title: Last-face delete skips the cascade opt-in
finding: rela delete ID@face without --cascade and MCP/Lua delete with cascade=false exempted face deletes from the relation refusal. Since RR-2466U1 deleting the last face also cuts inbound edges owned by other entities; the MCP success message also undercounted them.
severity: significant
resolution: 'DeleteEntityFace takes cascade and returns ErrHasRelations for a last face with edges when it is false. CLI (deleteTarget reports wholeEntity) and MCP (wholeEntityDelete) refuse and count with the family scope; Lua passes its cascade flag. Tests: TestDeleteEntityFace_LastFaceNeedsCascade and _NotLastIgnoresCascade and TestDeleteCmd_LastFaceNeedsCascade and TestHandleDeleteEntity_LastFaceNeedsCascade.'
status: addressed
---
