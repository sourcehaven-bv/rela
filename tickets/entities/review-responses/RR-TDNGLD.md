---
id: RR-TDNGLD
type: review-response
title: appFaceOrder set up in two phases
finding: faceOrder.app is assigned after the resolver is built, which needs a nil-app branch in of (cranky review).
severity: nit
reason: The visible reader is a required App field, so it must exist before the App. The nil branch returns token order only during construction, before any request.
status: wont-fix
---
