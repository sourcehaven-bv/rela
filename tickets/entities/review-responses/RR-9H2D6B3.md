---
id: RR-9H2D6B3
type: review-response
title: MCP clients cannot see which step form fields are required
finding: complete_flow_step/show_flow_run expose only waiting.fields. The server does not enforce form required/required_when (by design), so an agent can skip a field the form requires. Expose required fields in waiting or say in the tool description that only schema required is enforced.
severity: minor
resolution: The complete_flow_step description now says that rela validates values as any entity edit and does not check the form required or required_when. show_flow_run names waiting.form_problem. GUIDE-flows says the same.
status: addressed
---
