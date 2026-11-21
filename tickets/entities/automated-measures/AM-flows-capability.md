---
id: AM-flows-capability
type: automated-measure
title: 'Test: the SPA asks for the flow list only when the schema reports flows'
description: 'Guards against BUG-QTEEQZ4. TestSchemaCapabilities_Flows pins that /_schema reports capabilities.flows false without flows and true once SetFlows wires them. useFlowList.test.ts pins that the list is not asked for when the capability is false and that the decision waits for the schema.'
kind: test
location: internal/dataentry/flows_handler_test.go:TestSchemaCapabilities_Flows; frontend/src/composables/useFlowList.test.ts
status: active
---
