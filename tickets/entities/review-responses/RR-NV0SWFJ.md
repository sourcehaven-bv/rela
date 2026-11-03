---
id: RR-NV0SWFJ
type: review-response
title: Multi-document files lose later documents
finding: ApplyWithMetamodel reads only the first YAML document so migrate drops documents after ---. Predates this change.
severity: minor
reason: Pre-existing and outside this bug. Project config files are single-document.
status: deferred
---
