---
id: RR-9BJ9T8
type: review-response
title: Handwritten HTTP errors would drift from entity GET
finding: 'Security design review: the uniform 404 holds only if bodies match; bare faced addresses get 422 face_required elsewhere.'
severity: minor
resolution: 'Plan: map through writeWriteTargetError; test 404 body equality across hidden/absent/wrong-plural/not-wired.'
status: addressed
---
