---
id: RR-FYAKXX
type: review-response
title: Relation.Key() already exists as a string method
finding: Section 2.1 adds func (r *Relation) Key() RelationKey but entity.Relation.Key() string exists with about 88 non-test callers (map keys and log text). PR 2 would not compile as designed and the site estimate is wrong.
severity: significant
resolution: 'Amendment A1: the new method is Relation.Identity() RelationKey. Key() string is unchanged in Stage 2.'
status: addressed
---

## Finding

Section 2.1 adds func (r *Relation) Key() RelationKey but entity.Relation.Key()
string exists with about 88 non-test callers (map keys and log text). PR 2 would
not compile as designed and the site estimate is wrong.

Design: `.ignored/stage2-design.md` section 11.
