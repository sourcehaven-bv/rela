---
id: RR-T2DQV3D
type: review-response
title: 'Security: gate hops keep the ACL world'
finding: The AllFaces probe must apply to returned rows only; endpoint hops keep the request world.
severity: minor
reason: The probe only sets the result selection, as search admission does; hops carry their own Faces. Step 5 compiles the scope for one fixed world and tests a related(...) gate that matches only out of world.
status: deferred
---
