---
id: RR-XU9H33
type: review-response
title: RoleGrantsVerbOnFace precondition undocumented
finding: The doc should state that the caller passes a role from ForEntity/Globals.
severity: nit
resolution: The RoleGrantsVerbOnFace doc states roleName must be a role the principal holds as attributed by Request.ForEntity or Request.Globals, and that the method does not check this.
status: addressed
---
