---
id: AM-family-delete-authorizes-every-face
type: automated-measure
title: Family delete is denied when any face it removes is denied
description: A principal allowed to delete type@draft only must not delete a family that also has a published face; a grant on every face deletes every face, each with a version capture and audit record.
kind: test
location: internal/entitymanager (delete under a draft-only grant)
status: proposed
---
