---
id: RR-684C5KG
type: review-response
title: '[security] Lost access keeps receiving content for up to 10 minutes'
finding: '[security] Read and write access are checked only at the upgrade. A data change that hides the entity, a removed role relation or a membership change is only seen at the 10 minute lifetime cap (or JWT expiry), so a principal that lost read access keeps receiving keystrokes and a revoked writer keeps pushing updates.'
severity: significant
resolution: 'Each connection is rechecked every 30 seconds (read gate and update permission for writers) and closed when access is lost, connection lifetime is 2 minutes. Test: TestCollab_RecheckClosesLostAccess.'
status: addressed
---
