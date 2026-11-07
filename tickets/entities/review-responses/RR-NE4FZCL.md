---
id: RR-NE4FZCL
type: review-response
title: Stored roles followed an editable principal_property value
finding: Records were keyed by subject only. After a person entity lost its subject another user could set principal_property to it and a for_each child would carry the stored roles.
severity: significant
resolution: Records now name the user entity the subject resolved to at store time (IDPRoleWriter). Readers apply roles only to that entity while it alone holds the subject. Pinned by TestIDPRoleWriter_BindsTheResolvedEntity and the moved-value case in TestScheduledForEachPrincipal_StoredRoles.
status: addressed
---
