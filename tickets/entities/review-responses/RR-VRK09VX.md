---
id: RR-VRK09VX
type: review-response
title: 'Trigger ACL check: template variants not inspected'
finding: Properties a create.template variant supplies never pass the decidesAccess or editor checks.
severity: minor
resolution: 'A judged or reached trigger with a template variant is refused as unbounded. Test: "a template variant". The default template of a type applies to a direct create by the editor as well, so it is not judged separately.'
status: addressed
---
