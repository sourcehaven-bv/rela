---
id: FEAT-671KY1
type: feature
title: Framework-neutral data classification and policy applicability on the metamodel
summary: Declare which policies (GDPR, NIS2, internal confidentiality, ...) apply to entity types and properties, so rela can redact, erase, export and lint by classification.
description: An optional classification.yaml labels what data each field holds; labels carry an identifier role and combination rules derive labels for identifying field sets (ENISA ease of identification). Descriptive only. CLI tooling uses it for reports, coverage lint and advisory ACL audit findings; conformance profiles (org handling rules, e.g. GDPR) are checked in a later slice.
priority: medium
status: proposed
---
