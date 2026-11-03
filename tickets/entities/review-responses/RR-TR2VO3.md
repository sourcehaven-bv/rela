---
id: RR-TR2VO3
type: review-response
title: Verb helper must not silently refuse create and delete
finding: The planned core returned false for verbs other than read and update, a trap for the next caller. Route every write verb through verb.op() and GrantsVerbOnState, and update the Verb doc comment, which says it serves only who-can.
severity: minor
resolution: GrantsVerbOnFace routes create, update and delete through Verb.op to GrantsVerbOnState; the Verb doc comment now names the affordance resolver as a consumer.
status: addressed
---
