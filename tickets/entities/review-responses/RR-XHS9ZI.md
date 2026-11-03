---
id: RR-XHS9ZI
type: review-response
title: Deleted mirror stops every pull
finding: A trashed list or project mirror makes the create in mirror() fail the unique check, which aborts every pull.
severity: significant
resolution: 'mirror() guards the create with pcall and skips linking; test: a deleted list mirror does not stop the pull.'
status: addressed
---
