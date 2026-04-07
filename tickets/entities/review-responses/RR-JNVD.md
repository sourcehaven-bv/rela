---
id: RR-JNVD
type: review-response
title: TestMdMixedListBehavior asserts essentially nothing
finding: The test computes item1_type/item2_type but never asserts them. Only checks count==2, non-empty rendered, and contains 'task'/'plain'. Doesn't pin down actual goldmark behavior. A future goldmark upgrade that changes mixed-list semantics will pass silently.
severity: significant
resolution: 'TestMdMixedListBehavior now asserts the actual values: count==2, item1 is a task table with task=true, text=''task''; item2 is a plain string ''plain''; rendered output is exactly ''- [x] task\n- plain\n''. Goldmark''s mixed-list behavior is now empirically locked in.'
status: addressed
---
