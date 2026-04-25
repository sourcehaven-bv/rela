---
id: RR-LPQT1
type: review-response
title: Error message not actionable
finding: The error 'entity type X uses id_type=Y; custom ID Z not allowed (IDs are auto-generated)' describes the prohibition but gives no instruction. An LLM would retry with different IDs rather than realise it should omit id entirely.
severity: significant
resolution: Introduced customIDNotAllowedError in internal/workspace/workspace.go that emits '... not allowed -- omit the "id" field to auto-generate one (prefix "P-")'. The prefix hint makes the generated ID shape discoverable without a separate get_metamodel call.
status: addressed
---
