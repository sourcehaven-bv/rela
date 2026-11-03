---
id: RR-M22EZQ
type: review-response
title: Stored targets carry no kind, so render re-derives it from current config
finding: '`Message.Mentions []string` stores `jeroen` for a login and `USR-7` for a user id. render guesses the kind from the string''s shape plus today''s user type. When the deployment kind changes (adding principal_property or user_entity_type, see the critical finding), old login targets that pass entity.ValidateID get resolved as entity ids. They can render as a user link to an unrelated entity named `jeroen`. Old id targets would render as `@USR-7` logins. Fix: store the kind with each target (`[]channels.Mention{Target, Kind}`, with kind user, job or login), set by classifySpans. render then trusts the post''s decision, which is the stated reason for storing Mentions at all.'
severity: minor
resolution: Mentions stored as []channels.Mention{Target, Form, Span}; render trusts stored form; TestRender_Mentions row for mapping switched on later.
status: addressed
---
