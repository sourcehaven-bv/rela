---
id: RR-Z4QBXC
type: review-response
title: bareref guard does not cover RelationKey literals
finding: A RelationKey literal without FromFace is the relation form of the bare-ref mistake and the section 6.2 guard does not flag it.
severity: minor
reason: Identity-scoped edges legitimately have an empty FromFace and are the majority of relation writes. A guard would be almost all allowlist noise. Review and the PR 7 relation tests cover tailed edges.
status: wont-fix
---

## Finding

A RelationKey literal without FromFace is the relation form of the bare-ref
mistake and the section 6.2 guard does not flag it.

Design: `.ignored/stage2-design.md` section 11.
