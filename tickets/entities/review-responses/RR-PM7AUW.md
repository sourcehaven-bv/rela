---
id: RR-PM7AUW
type: review-response
title: gatedEntity reports every read failure as not found
finding: A store outage or cancelled context answered 'entity not found', which may lead an agent to create a duplicate.
severity: minor
resolution: gatedEntity maps only store.ErrNotFound to not found; other errors are logged and answered 'reading the entity failed'.
status: addressed
---
