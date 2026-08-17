---
id: AM-validate-fails-on-unparseable-entity
type: automated-measure
title: rela validate exits non-zero when an entity cannot be parsed
description: Guards the failure mode where a malformed entity is silently dropped from the result set and validate still exits 0. A fixture entity with unparseable frontmatter must make validate exit non-zero and name the offending path.
kind: ci
location: internal/cli/validate.go + tickets CI job in .github/workflows/ci.yml
status: proposed
---
