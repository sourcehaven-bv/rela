---
id: RR-G3QH9AN
type: review-response
title: rela flow validate does not check triggers
finding: 'internal/cli/flows.go validateFile only calls LoadFlows, so a flows.yaml with a broken triggers: block is reported valid. Fix: run appbuild.CheckTriggers and include the trigger count.'
severity: significant
resolution: FlowValidateCmd.validateFile runs appbuild.CheckTriggers and reports the trigger count. TestFlowValidate_Run has a valid-trigger and a broken-trigger case.
status: addressed
---
