---
id: RR-S57CAY
type: review-response
title: 'fields: all into the bare face is a full-record replace, and check (1) does not bound field disclosure'
finding: 'Two related doc inaccuracies, neither introduced by this change but both made reachable by it. (a) buildCopyTarget replaces target.Properties wholesale under AllFields, so a property living only on the target face is dropped. With bare_face: published the target is also the row ordinary writes and unique: keys live on, so a promote can blank a natural key. (b) The doc comment leaned on check (1) to bound the fields-all disclosure, but PermitsReadFace is a row-and-face verdict, not a field one, so it never made that guarantee.'
severity: minor
resolution: 'Corrected the authorizeCopy doc comment to say plainly that check (1) does NOT bound field disclosure and to note the full-replace semantics. Added a paragraph to docs/content-states.md warning that fields: all is a replace rather than a merge, with the bare_face: published case called out and the mitigation (carry the property on the source face, or map fields explicitly).'
status: addressed
---
