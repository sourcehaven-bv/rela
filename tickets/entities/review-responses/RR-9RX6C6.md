---
id: RR-9RX6C6
type: review-response
title: ListAddressed carries up to 2000 full message docs through the outer sort just to get author and type
finding: 'The outer query joins `channel_messages` and selects `m.doc` before `ORDER BY ... LIMIT ?`. The engine therefore materializes and sorts up to 2000 docs of up to MaxMessageBytes (16 KiB) each, which is about 32 MB worst case. It then JSON-decodes up to 500 docs, message bodies included, on every inbox call and every watch replay. The file tier already denormalizes author, type and reply_to into mentions.json, so the backends are also inconsistent. Fix: add `author_user`, `author_tool`, `author_org`, `author_type`, `msg_type` and `reply_to` columns to channel_mentions. They are immutable for the row''s life: an edit never changes author or type, and a removal deletes the row. Drop the join to channel_messages (a removal already deletes the rows). Or move the doc join outside a LIMITed derived table. The budget and explain tests should cover the chosen shape.'
severity: significant
resolution: Added immutable author_user/tool/org/type, msg_type, reply_to columns; ListAddressed drops the channel_messages join and decodes no docs; AC 13 budget asserts it.
status: addressed
---
