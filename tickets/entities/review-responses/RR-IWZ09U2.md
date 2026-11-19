---
id: RR-IWZ09U2
type: review-response
title: Id reuse merges another entity's records
finding: audit_subjects is keyed by bare id, so delete-and-recreate or rename-then-reuse shows the earlier entity's records under the new one.
severity: significant
resolution: 'Plan: new migration adds face and starts columns to audit_subjects. starts marks a record that begins a lifecycle of that id and face (create, or rename/convert into it). Every page reads from the latest start; entity records must match the URL type. Tests for both reuse cases.'
status: addressed
---

audit_subjects is keyed by bare id, so delete-and-recreate or rename-then-reuse
shows the earlier entity's records under the new one.
