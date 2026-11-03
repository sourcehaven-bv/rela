---
id: RR-IN8DMD
type: review-response
title: ConditionallyVisible godoc argues against per-principal filterability
finding: The policy-wide method's godoc says a per-principal filterability answer is an oracle. The new per-principal method contradicts it without comment; add a sentence that the answer depends only on config and the caller's own roles, never on rows.
severity: minor
resolution: 'Added a paragraph to acl.Policy.ConditionallyVisible: a list filter may be judged per principal because the answer depends only on the policy and the caller''s own roles.'
status: addressed
---
