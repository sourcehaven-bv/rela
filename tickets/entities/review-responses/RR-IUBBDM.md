---
id: RR-IUBBDM
type: review-response
title: Zero expiry on the context ends every stream at once
finding: 'internal/dataentry/watcher.go:425-428 treats a present-but-zero expiry as expired, because time.Now().Before(time.Time{}) is false. Only the IsZero guard in internal/dataentry/jwtgate.go:190 stops that, and no test covers a gated stream whose verifier returns a zero ExpiresAt (TestSSEExpiry_NoGateStaysOpen has no gate, so the branch is never taken). Deleting that guard would close every JWT stream at once with all tests green. Make credentialExpiry return ok=false for a zero time, or drop hasExpiry and test !expiry.IsZero() in handleSSE, and add a test case: gate installed, verifier returns zero ExpiresAt, stream stays open.'
severity: minor
resolution: credentialExpiry returns ok=false for a zero time; the "gate with zero expiry" row in TestSSEExpiry_NoExpiryStaysOpen sends a zero expiry through the real gate and fails without the check.
status: addressed
---
