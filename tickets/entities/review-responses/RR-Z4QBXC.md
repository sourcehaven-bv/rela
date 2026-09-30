---
id: RR-Z4QBXC
type: review-response
title: bareref guard does not cover RelationKey literals
finding: A RelationKey literal without FromFace is the relation form of the bare-ref mistake and the section 6.2 guard does not flag it.
severity: minor
resolution: PR 7 (#1733) added internal/archguard/tailless_test.go. It flags an entity.RelationKey literal that sets From without a FromFace and pins the existing sites in a shrink-only taillessKeyAllowlist with a reason per file.
reason: Identity-scoped edges legitimately have an empty FromFace and are the majority of relation writes. A guard would be almost all allowlist noise. Review and the PR 7 relation tests cover tailed edges.
status: addressed
---

## Finding

A RelationKey literal without FromFace is the relation form of the bare-ref
mistake and the section 6.2 guard does not flag it.

Design: `.ignored/stage2-design.md` section 11.
