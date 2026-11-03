---
id: RR-3VUPJ0
type: review-response
title: Descriptor label contract is inconsistent
finding: 'realtimecfg defaults channel type, message type, action, role and now lifecycle labels to the name (validate.go orDefault, including the new lifecycleOp call), but a property label stays '''' when unset. The OpenAPI schema marks `label` required for all of them. The SPA must therefore treat '''' as ''use the name'' for properties and can never tell ''authored'' from ''defaulted'' for the rest. That contradicts the plan''s ''labels are authored, never derived; with no label the SPA shows the name''. Pick one: send '''' everywhere when unauthored and let the SPA fall back, or default everywhere, and document it on ChannelPropertySchema. Note too that the lifecycle Gate.Label changed from '''' to the op name, so any message that formats Gate.Label now reads differently. Check gateDenied output.'
severity: minor
resolution: The label contract (never empty, defaulting to the name, except create_label) is documented on the apiwire types, the OpenAPI schema and the TypeScript mirror, and describe_test checks that labels are non-empty.
status: addressed
---
