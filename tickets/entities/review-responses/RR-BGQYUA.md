---
id: RR-BGQYUA
type: review-response
title: Arity must also filter getEntity
finding: feedEntitySource.getEntity takes three arguments (type, id) and is not the ungated entityReader.getEntity; counting it by name would pin an unrelated method.
severity: minor
resolution: getEntity is counted only with exactly two arguments, like GetEntity.
status: addressed
---
