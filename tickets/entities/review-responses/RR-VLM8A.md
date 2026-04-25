---
id: RR-VLM8A
type: review-response
title: Use EqualFold instead of ToLower for isSafeReturnPath case-fold
finding: return_path.go:35-40 uses strings.ToLower(s[:4]) then compares to constants. Safe for UTF-8 at byte offsets but allocates on uppercase input and invites paranoid-debugging wonder about Unicode. strings.EqualFold(s[:4], "/%5C") || strings.EqualFold(s[:4], "/%2F") expresses intent directly, no allocation, self-documenting.
severity: minor
resolution: Replaced the ToLower switch with two strings.EqualFold calls comparing against /%5C and /%2F. Behavior identical, no string allocation, clearer intent.
status: addressed
---
