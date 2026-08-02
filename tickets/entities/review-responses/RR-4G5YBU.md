---
id: RR-4G5YBU
type: review-response
title: MIME allowlist runs before transform; output type never re-checked
finding: PolicyProcessor runs the MIME allowlist BEFORE transforms (policy.go:48 then :62), validating the attacker's INPUT bytes, never the re-encoded OUTPUT. The AC 'output stays within allow' is not enforced by current ordering. Open Q4 raised it but left it unresolved.
severity: significant
resolution: 'Decision: the re-encode target is constrained to a fixed safe set (png/jpeg) that is always within default-safe, so output is safe by construction; AND we re-validate the output filename''s extension against the property allowlist after a native step (cheap, no re-sniff needed since we control the encoder). Documented the ordering argument in the plan; removed the vague AC and replaced with an explicit ''reencode target must be in the effective allowlist, validated at metamodel load'' rule.'
status: addressed
---
