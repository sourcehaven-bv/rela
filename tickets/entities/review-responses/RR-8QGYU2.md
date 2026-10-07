---
id: RR-8QGYU2
type: review-response
title: Cascade refusal is an existence signal
finding: Refusing the parent delete because of a child the principal cannot delete tells them such a child exists. Entity existence is treated as secret. authorizeCascadeRelations already has the same one-bit channel for hidden edges, so this is parity, but the plan should state it and test that the error text carries no child id or title.
severity: minor
resolution: 'Plan: refusal text carries no child id or title; documented as parity with authorizeCascadeRelations; ACL test asserts the text.'
status: addressed
---
