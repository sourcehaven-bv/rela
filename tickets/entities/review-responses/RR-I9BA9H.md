---
id: RR-I9BA9H
type: review-response
title: A cached acl.Request could survive the principal swap
finding: Read-side consumers reuse a ctx Request without checking its principal. Nothing attaches one before /hooks today.
severity: significant
resolution: handle() clears the ctx Request slot after the swap (acl.WithRequest(ctx, nil)) with a test. Transition guards on the hook path still fail closed (pre-existing); documented.
status: addressed
---
